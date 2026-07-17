package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	RepoOwner = "YeDaChai"
	RepoName  = "tun-tui"
	AppName   = "tun-tui"
)

// Release is the subset of GitHub release JSON we need.
type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Info is the result of a version check for the current platform.
type Info struct {
	Current     string
	Latest      string
	Newer       bool
	DownloadURL string
	AssetName   string
}

func latestAPI() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", RepoOwner, RepoName)
}

// Check compares the running version against GitHub latest release.
func Check(current string) (Info, error) {
	rel, err := fetchLatest()
	if err != nil {
		return Info{}, err
	}
	latest := Normalize(rel.TagName)
	current = Normalize(current)
	asset, url, err := pickAsset(rel, latest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Info{}, err
	}
	return Info{
		Current:     current,
		Latest:      latest,
		Newer:       IsNewer(latest, current),
		DownloadURL: url,
		AssetName:   asset,
	}, nil
}

// Apply downloads Info.DownloadURL and replaces the running executable.
func Apply(info Info) error {
	if info.DownloadURL == "" {
		return fmt.Errorf("没有可下载的更新包")
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位当前程序: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	tmpDir, err := os.MkdirTemp("", "tun-tui-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, filepath.Base(info.AssetName))
	if err := downloadFile(info.DownloadURL, archivePath); err != nil {
		return err
	}

	binName := AppName
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	extracted, err := extractBinary(archivePath, tmpDir, binName)
	if err != nil {
		return err
	}
	return replaceExecutable(exe, extracted)
}

func fetchLatest() (Release, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, latestAPI(), nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", AppName)

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("请求 GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Release{}, fmt.Errorf("GitHub API %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("解析 release: %w", err)
	}
	if rel.TagName == "" {
		return Release{}, fmt.Errorf("release 缺少 tag_name")
	}
	return rel, nil
}

func pickAsset(rel Release, version, goos, goarch string) (name, url string, err error) {
	cands, err := archiveCandidates(version, goos, goarch)
	if err != nil {
		return "", "", err
	}
	byName := make(map[string]string, len(rel.Assets))
	for _, a := range rel.Assets {
		if a.Name != "" && a.BrowserDownloadURL != "" {
			byName[a.Name] = a.BrowserDownloadURL
		}
	}
	for _, want := range cands {
		if u, ok := byName[want]; ok {
			return want, u, nil
		}
	}

	// Fallback: match platform label inside any asset name (covers odd version prefixes).
	label, ext, err := platformLabel(goos, goarch)
	if err != nil {
		return "", "", err
	}
	suffix := "-" + label + ext
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, suffix) && a.BrowserDownloadURL != "" {
			return a.Name, a.BrowserDownloadURL, nil
		}
	}
	return "", "", fmt.Errorf("最新版本没有适合本机的包（%s/%s），期望如: %s", goos, goarch, cands[0])
}

// PlatformArchive matches current Makefile / install.sh release names.
func PlatformArchive(version string) (string, error) {
	return platformArchive(version, runtime.GOOS, runtime.GOARCH)
}

func platformArchive(version, goos, goarch string) (string, error) {
	version = Normalize(version)
	if version == "" {
		return "", fmt.Errorf("版本号为空")
	}
	label, ext, err := platformLabel(goos, goarch)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s%s", AppName, version, label, ext), nil
}

// archiveCandidates lists preferred asset names, including legacy aliases.
func archiveCandidates(version, goos, goarch string) ([]string, error) {
	primary, err := platformArchive(version, goos, goarch)
	if err != nil {
		return nil, err
	}
	out := []string{primary}
	version = Normalize(version)
	switch {
	case goos == "darwin" && goarch == "arm64":
		out = append(out, fmt.Sprintf("%s-%s-macos-apple-silicon.tar.gz", AppName, version))
	case goos == "darwin" && goarch == "amd64":
		out = append(out, fmt.Sprintf("%s-%s-macos-intel.tar.gz", AppName, version))
	case goos == "windows" && goarch == "amd64":
		// Older experimental names if any.
		out = append(out, fmt.Sprintf("%s-%s-windows-amd64.zip", AppName, version))
	}
	return out, nil
}

func platformLabel(goos, goarch string) (label, ext string, err error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64":
			return "macos-apple-silicon-arm64", ".tar.gz", nil
		case "amd64":
			return "macos-intel-x86_64", ".tar.gz", nil
		}
		return "", "", fmt.Errorf("不支持的 macOS 架构: %s", goarch)
	case "linux":
		switch goarch {
		case "amd64":
			return "linux-x86_64", ".tar.gz", nil
		case "arm64":
			return "linux-arm64", ".tar.gz", nil
		}
		return "", "", fmt.Errorf("不支持的 Linux 架构: %s", goarch)
	case "windows":
		switch goarch {
		case "amd64":
			return "windows-x86_64", ".zip", nil
		}
		return "", "", fmt.Errorf("不支持的 Windows 架构: %s（仅 x86_64）", goarch)
	default:
		return "", "", fmt.Errorf("不支持的系统: %s/%s", goos, goarch)
	}
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", AppName)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractBinary(archivePath, destDir, binName string) (string, error) {
	switch {
	case strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz"),
		strings.HasSuffix(strings.ToLower(archivePath), ".tgz"):
		return extractTarGz(archivePath, destDir, binName)
	case strings.HasSuffix(strings.ToLower(archivePath), ".zip"):
		return extractZip(archivePath, destDir, binName)
	default:
		return "", fmt.Errorf("未知压缩格式: %s", filepath.Base(archivePath))
	}
}

func isAppBinaryName(base, binName string) bool {
	return base == binName ||
		base == AppName ||
		base == AppName+".exe"
}

func extractTarGz(path, destDir, binName string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		base := filepath.Base(hdr.Name)
		if !isAppBinaryName(base, binName) {
			continue
		}
		out := filepath.Join(destDir, binName)
		if err := writeFileFrom(out, tr, 0o755); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("压缩包内未找到 %s", binName)
}

func extractZip(path, destDir, binName string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if !isAppBinaryName(base, binName) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out := filepath.Join(destDir, binName)
		err = writeFileFrom(out, rc, 0o755)
		rc.Close()
		if err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("压缩包内未找到 %s", binName)
}

func writeFileFrom(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func replaceExecutable(target, source string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o755)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode()
	}

	dir := filepath.Dir(target)
	tmp := filepath.Join(dir, "."+filepath.Base(target)+".new")
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("写入更新文件失败（需要写权限）: %w", err)
	}

	backup := target + ".old"
	_ = os.Remove(backup)

	// Rename-away then rename-in works on Unix and Windows (running image can be renamed).
	if err := os.Rename(target, backup); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("备份旧版本失败: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(backup, target)
		_ = os.Remove(tmp)
		return fmt.Errorf("替换程序失败: %w", err)
	}
	_ = os.Remove(backup) // may fail on Windows while still running; fine
	if runtime.GOOS == "darwin" {
		clearQuarantine(target)
	}
	return nil
}

func clearQuarantine(path string) {
	_ = exec.Command("xattr", "-d", "com.apple.quarantine", path).Run()
}

// Normalize strips a leading "v" and whitespace.
func Normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// IsNewer reports whether latest is a higher semver-ish version than current.
// Non-numeric current (e.g. "dev") is treated as older when latest parses.
func IsNewer(latest, current string) bool {
	latest, current = Normalize(latest), Normalize(current)
	if latest == "" || latest == current {
		return false
	}
	if !isSemver(current) {
		return isSemver(latest)
	}
	return compareVersion(latest, current) > 0
}

func isSemver(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		p = strings.SplitN(p, "-", 2)[0]
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func compareVersion(a, b string) int {
	as := versionParts(a)
	bs := versionParts(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(as) {
			ai = as[i]
		}
		if i < len(bs) {
			bi = bs[i]
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.SplitN(v, "-", 2)[0]
	raw := strings.Split(v, ".")
	out := make([]int, 0, len(raw))
	for _, p := range raw {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}
