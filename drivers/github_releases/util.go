package github_releases

import (
	"fmt"
	"strings"
)

// 解析挂载结构
func (d *GithubReleases) ParseRepos(text string) ([]MountPoint, error) {
	lines := strings.Split(text, "\n")
	points := make([]MountPoint, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		path, repo := "", ""
		if len(parts) == 1 {
			path = "/"
			repo = parts[0]
		} else if len(parts) == 2 {
			path = fmt.Sprintf("/%s", strings.Trim(parts[0], "/"))
			repo = parts[1]
		} else {
			return nil, fmt.Errorf("invalid format: %s", line)
		}

		points = append(points, MountPoint{
			Point: path,
			Repo:  repo,
		})
	}
	d.points = points
	return points, nil
}

// 获取下一级目录
func GetNextDir(wholePath string, basePath string) string {
	basePath = fmt.Sprintf("%s/", strings.TrimRight(basePath, "/"))
	if !strings.HasPrefix(wholePath, basePath) {
		return ""
	}
	remainingPath := strings.TrimLeft(strings.TrimPrefix(wholePath, basePath), "/")
	if remainingPath != "" {
		parts := strings.Split(remainingPath, "/")
		nextDir := parts[0]
		if strings.HasPrefix(wholePath, strings.TrimRight(basePath, "/")+"/"+nextDir) {
			return nextDir
		}
	}
	return ""
}
