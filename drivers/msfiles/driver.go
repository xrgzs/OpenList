package msfiles

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/drivers/virtual"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/antchfx/htmlquery"
)

type MSFiles struct {
	model.Storage
	Addition
}

func (d *MSFiles) Config() driver.Config {
	return config
}

func (d *MSFiles) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *MSFiles) Init(ctx context.Context) error {
	// TODO login / refresh token
	//op.MustSaveDriverStorage(d)
	return nil
}

func (d *MSFiles) Drop(ctx context.Context) error {
	return nil
}

func (d *MSFiles) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	fmt.Println("https://files.rg-adguard.net/" + dir.GetID())
	req, err := http.NewRequest(http.MethodGet, "https://files.rg-adguard.net/"+dir.GetID(), nil)
	if err != nil {
		return nil, err
	}
	for h := range COMMON_HEADERS {
		req.Header.Set(h, COMMON_HEADERS[h])
	}
	res, err := base.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	doc, err := htmlquery.Parse(res.Body)
	if err != nil {
		return nil, err
	}
	nodes := htmlquery.Find(doc, `//table[@class='info']/tbody/tr[position() > 1]/td/a`)
	fmt.Println(nodes)
	var o []model.Obj
	for _, n := range nodes {
		// get text
		name := htmlquery.InnerText(n)
		if name == "" {
			continue
		}
		fmt.Println(n, name)

		// get id from href
		href := htmlquery.SelectAttr(n, "href")
		if href == "" || !strings.HasPrefix(href, "https://files.rg-adguard.net/") {
			continue
		}
		// extract id from href
		id := strings.TrimPrefix(href, "https://files.rg-adguard.net/")

		if strings.HasPrefix(id, "file/") {
			// file, need to extract file info by requesting file link recursively
			req, err := http.NewRequest(http.MethodGet, href, nil)
			if err != nil {
				return nil, err
			}
			for h := range COMMON_HEADERS {
				req.Header.Set(h, COMMON_HEADERS[h])
			}
			res, err := base.HttpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer res.Body.Close()
			doc, err := htmlquery.Parse(res.Body)
			if err != nil {
				return nil, err
			}
			rows := htmlquery.Find(doc, `//table[@class='info']/tbody/tr[td[@class='text'] and td[@class='desc']]`)

			var i = Object{
				Object: model.Object{
					ID:       id,
					Name:     name,
					IsFolder: false,
				},
				CanDlOfficial: func() bool {
					downloadLink := htmlquery.FindOne(doc, `//input[@name="dl_official"]`)
					return downloadLink != nil
				}(),
			}
			h := make(map[*utils.HashType]string)
			for _, row := range rows {
				// 提取键列文本（td.text 的内容）
				keyNode := htmlquery.FindOne(row, "./td[@class='text']")
				if keyNode == nil {
					continue
				}
				key := strings.TrimSpace(htmlquery.InnerText(keyNode))
				// 清洗键：去掉末尾的冒号（比如 "File:" → "File"）
				key = strings.TrimSuffix(key, ":")
				if key == "" {
					continue
				}

				// 提取值列文本（td.desc 的内容）
				valueNode := htmlquery.FindOne(row, "./td[@class='desc']")
				if valueNode == nil {
					continue
				}
				value := strings.TrimSpace(htmlquery.InnerText(valueNode))
				if value == "" {
					continue
				}

				fmt.Printf("key: %s, value: %s\n", key, value)
				switch key {
				case "File":
					i.Name = value
				case "Size":
					// 24.551 MB (25743763 bytes)
					parts := strings.Split(value, "(")
					if len(parts) != 2 {
						continue
					}
					sizePart := strings.TrimSpace(parts[1])
					sizePart = strings.TrimSuffix(sizePart, " bytes)")
					size, err := strconv.ParseInt(sizePart, 10, 64)
					if err != nil {
						continue
					}
					i.Size = size
				case "MD5":
					h[utils.MD5] = value
				case "SHA-1":
					h[utils.SHA1] = value
				case "SHA-256":
					h[utils.SHA256] = value
				}
			}
			i.HashInfo = utils.NewHashInfoByMap(h)
			o = append(o, &i)
			continue
		}
		o = append(o, &model.Object{
			ID:       id,
			Name:     name,
			IsFolder: true,
		})
	}
	return o, nil
}

func (d *MSFiles) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if file.(*Object).CanDlOfficial {
		var url string
		// get the url after redirect
		req := base.NoRedirectClient.R()
		// set headers
		req.SetHeaders(COMMON_HEADERS)
		req.SetHeader("Content-Type", "application/x-www-form-urlencoded")
		req.SetBody("dl_official=Download+the+file+directly+from+the+Microsoft+server")
		req.SetDoNotParseResponse(true)
		res, err := req.Post("https://files.rg-adguard.net/" + file.GetID())
		if err != nil {
			return nil, err
		}
		_ = res.RawResponse.Body.Close()
		if (res.StatusCode() == 302 || res.StatusCode() == 307 || res.StatusCode() == 308) && res.Header().Get("location") != "" {
			url = res.Header().Get("location")
		} else {
			return nil, fmt.Errorf("redirect failed, status: %d", res.StatusCode())
		}
		if url == "" {
			return nil, fmt.Errorf("empty download url")
		}
		return &model.Link{
			URL: url,
		}, nil
	}
	if !args.Redirect {
		// to rapid upload, we return a fake link
		if d.Addition.DummyFile {
			return &model.Link{
				RangeReader: stream.GetRangeReaderFromMFile(file.GetSize(), virtual.DummyMFile{}),
			}, nil
		}
		return &model.Link{
			URL: "https://files.rg-adguard.net/" + file.GetID(),
		}, nil
	}
	return nil, errs.NotImplement
}

func (d *MSFiles) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
	// TODO return storage details (total space, free space, etc.)
	return nil, errs.NotImplement
}

var _ driver.Driver = (*MSFiles)(nil)
