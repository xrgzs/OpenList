package _123_open

import (
	"context"
	"fmt"

	_123_open "github.com/OpenListTeam/OpenList/v4/drivers/123_open"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/setting"

	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/offline_download/tool"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Open123 struct {
}

func (o *Open123) Name() string {
	return "123 Open"
}

func (o *Open123) Items() []model.SettingItem {
	return nil
}

func (o *Open123) Run(task *tool.DownloadTask) error {
	return errs.NotSupport
}

func (o *Open123) Init() (string, error) {
	return "ok", nil
}

func (o *Open123) IsReady() bool {
	tempDir := setting.GetStr(conf.Pan123OpenTempDir)
	if tempDir == "" {
		return false
	}
	storage, _, err := op.GetStorageAndActualPath(tempDir)
	if err != nil {
		return false
	}
	if _, ok := storage.(*_123_open.Open123); !ok {
		return false
	}
	return true
}

func (o *Open123) AddURL(args *tool.AddUrlArgs) (string, error) {
	storage, actualPath, err := op.GetStorageAndActualPath(args.TempDir)
	if err != nil {
		return "", err
	}
	driver123Open, ok := storage.(*_123_open.Open123)
	if !ok {
		return "", fmt.Errorf("unsupported storage driver for offline download, only 123 Cloud is supported")
	}

	ctx := context.Background()

	if err := op.MakeDir(ctx, storage, actualPath); err != nil {
		return "", err
	}

	parentDir, err := op.GetUnwrap(ctx, storage, actualPath)
	if err != nil {
		return "", err
	}

	hashs, err := driver123Open.OfflineDownload(ctx, []string{args.Url}, parentDir)
	if err != nil || len(hashs) < 1 {
		return "", fmt.Errorf("failed to add offline download task: %w", err)
	}

	return hashs[0], nil
}

func (o *Open123) Remove(task *tool.DownloadTask) error {
	return errs.NotSupport
}

func (o *Open123) Status(task *tool.DownloadTask) (*tool.Status, error) {
	storage, _, err := op.GetStorageAndActualPath(task.TempDir)
	if err != nil {
		return nil, err
	}
	driver123Open, ok := storage.(*_123_open.Open123)
	if !ok {
		return nil, fmt.Errorf("unsupported storage driver for offline download, only 123 Open is supported")
	}

	tasks, err := driver123Open.OfflineList(context.Background())
	if err != nil {
		return nil, err
	}

	s := &tool.Status{
		Progress:  0,
		NewGID:    "",
		Completed: false,
		Status:    "the task has been deleted",
		Err:       nil,
	}

	for _, t := range tasks.Tasks {
		if t.InfoHash == task.GID {
			s.Progress = float64(t.PercentDone)
			s.Status = t.GetStatus()
			s.Completed = t.IsDone()
			s.TotalBytes = t.Size
			if t.IsFailed() {
				s.Err = fmt.Errorf(t.GetStatus())
			}
			return s, nil
		}
	}
	s.Err = fmt.Errorf("the task has been deleted")
	return nil, nil
}

var _ tool.Tool = (*Open123)(nil)

func init() {
	tool.Tools.Add(&Open123{})
}
