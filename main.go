package main

import (
	"archive/zip"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

//go:embed web/*
var webFiles embed.FS

type appConfig struct {
	Port      int
	UploadDir string
}

type pageData struct {
	Port      int
	Addresses []string
}

type fileItem struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

type jsonResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

var cfg appConfig
var uploadRoot string

func main() {
	flag.IntVar(&cfg.Port, "port", 0, "服务监听端口，0 表示随机可用端口")
	flag.StringVar(&cfg.UploadDir, "dir", "uploads", "上传文件保存目录")
	flag.Parse()

	root, err := filepath.Abs(cfg.UploadDir)
	if err != nil {
		log.Fatalf("获取上传目录失败: %v", err)
	}
	uploadRoot = filepath.Clean(root)

	if err := os.MkdirAll(uploadRoot, 0755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/static/", staticHandler)
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/files", filesHandler)
	mux.HandleFunc("/download", downloadHandler)
	mux.HandleFunc("/preview", previewHandler)
	mux.HandleFunc("/delete", deleteHandler)
	mux.HandleFunc("/qr", qrHandler)

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", cfg.Port))
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
	}
	cfg.Port = actualListenPort(listener)

	addresses := localAccessURLs(cfg.Port)
	printStartupInfo(addresses)
	mux.HandleFunc("/", indexHandler(addresses))

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// 大文件上传不设置 ReadTimeout，避免慢速局域网上传被中断。
	}

	log.Printf("正在监听 0.0.0.0:%d，文件保存目录: %s", cfg.Port, uploadRoot)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func actualListenPort(listener net.Listener) int {
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return cfg.Port
	}
	return tcpAddr.Port
}

func printStartupInfo(addresses []string) {
	fmt.Println("========================================")
	fmt.Println("Lan File Transfer 已启动")
	fmt.Println("本机局域网 IP / 访问地址:")
	for _, addr := range addresses {
		fmt.Printf("  %s\n", addr)
	}
	fmt.Println("手机和电脑连接同一局域网后，用浏览器打开上面的地址。")
	fmt.Println("========================================")
}

func localAccessURLs(port int) []string {
	ips := localIPv4s()
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}

	addresses := make([]string, 0, len(ips))
	for _, ip := range ips {
		addresses = append(addresses, fmt.Sprintf("http://%s:%d", ip, port))
	}
	return addresses
}

func localIPv4s() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var ips []string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			continue
		}
		value := ip.String()
		if !seen[value] {
			seen[value] = true
			ips = append(ips, value)
		}
	}
	sort.Slice(ips, func(i, j int) bool {
		left := net.ParseIP(ips[i]).To4()
		right := net.ParseIP(ips[j]).To4()
		leftPrivate := left != nil && left.IsPrivate()
		rightPrivate := right != nil && right.IsPrivate()
		if leftPrivate != rightPrivate {
			return leftPrivate
		}
		return ips[i] < ips[j]
	})
	return ips
}

func indexHandler(addresses []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tplBytes, err := webFiles.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "index not found", http.StatusInternalServerError)
			return
		}

		tpl, err := template.New("index").Parse(string(tplBytes))
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.Execute(w, pageData{Port: cfg.Port, Addresses: addresses})
	}
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.StripPrefix("/static/", http.FileServer(http.FS(webFiles))).ServeHTTP(w, r)
}

func qrHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	text := strings.TrimSpace(r.URL.Query().Get("text"))
	if text == "" {
		http.Error(w, "missing text", http.StatusBadRequest)
		return
	}
	if len(text) > 2048 {
		http.Error(w, "text too long", http.StatusBadRequest)
		return
	}

	// 使用成熟二维码库生成 PNG，避免前端手写二维码在手机扫码器上兼容性不足。
	png, err := qrcode.Encode(text, qrcode.Medium, 320)
	if err != nil {
		http.Error(w, "qr encode failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, jsonResponse{OK: false, Message: "仅支持 POST"})
		return
	}

	reader, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: "请求不是 multipart/form-data"})
		return
	}

	// 前端按 path -> file 的顺序发送。这里用队列把相对路径和文件流对应起来。
	pendingPaths := make([]string, 0)
	savedCount := 0

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: "读取上传流失败"})
			return
		}

		switch part.FormName() {
		case "path":
			value, err := io.ReadAll(io.LimitReader(part, 32*1024))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: "读取文件路径失败"})
				return
			}
			pendingPaths = append(pendingPaths, string(value))
		case "file", "files":
			rel := part.FileName()
			if len(pendingPaths) > 0 {
				rel = pendingPaths[0]
				pendingPaths = pendingPaths[1:]
			}
			if rel == "" {
				writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: "缺少文件名"})
				return
			}
			if err := saveUploadedFile(rel, part); err != nil {
				writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: err.Error()})
				return
			}
			savedCount++
		}
		_ = part.Close()
	}

	if savedCount == 0 {
		writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: "没有收到文件"})
		return
	}
	writeJSON(w, http.StatusOK, jsonResponse{OK: true, Message: fmt.Sprintf("已上传 %d 个文件", savedCount)})
}

func saveUploadedFile(rel string, src io.Reader) error {
	cleanRel, err := cleanRelativePath(rel)
	if err != nil {
		return err
	}
	if strings.HasSuffix(cleanRel, "/") {
		return errors.New("上传路径不能是目录")
	}

	dest, err := secureJoin(cleanRel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	file, finalPath, err := createUniqueFile(dest)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// io.Copy 直接从请求流写入磁盘，不会把大文件一次性读入内存。
	if _, err := io.Copy(file, src); err != nil {
		_ = os.Remove(finalPath)
		return fmt.Errorf("保存文件失败: %w", err)
	}
	return nil
}

func createUniqueFile(target string) (*os.File, string, error) {
	dir := filepath.Dir(target)
	name := filepath.Base(target)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for i := 0; i < 10000; i++ {
		candidate := target
		if i > 0 {
			candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		}
		file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			return file, candidate, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("重名文件过多")
}

func filesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, jsonResponse{OK: false, Message: "仅支持 GET"})
		return
	}

	items := make([]fileItem, 0)
	err := filepath.WalkDir(uploadRoot, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == uploadRoot {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(uploadRoot, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		size := info.Size()
		if entry.IsDir() {
			size = 0
		}
		items = append(items, fileItem{
			Path:    rel,
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    size,
			ModTime: info.ModTime().Format(time.RFC3339),
		})
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonResponse{OK: false, Message: "读取文件列表失败"})
		return
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path)
	})

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(items)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target, rel, err := requestPath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "stat failed", http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		downloadZip(w, r, target, rel)
		return
	}

	name := filepath.Base(target)
	w.Header().Set("Content-Disposition", contentDisposition(name))
	if ctype := mime.TypeByExtension(filepath.Ext(name)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}

	file, err := os.Open(target)
	if err != nil {
		http.Error(w, "open failed", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// ServeContent 支持 Range 请求，下载大文件时会从磁盘流式发送。
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func previewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target, _, err := requestPath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "stat failed", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "folders cannot be previewed", http.StatusBadRequest)
		return
	}

	name := filepath.Base(target)
	if ctype := mime.TypeByExtension(filepath.Ext(name)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	w.Header().Set("Content-Disposition", inlineContentDisposition(name))

	file, err := os.Open(target)
	if err != nil {
		http.Error(w, "open failed", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 预览也使用 ServeContent，支持视频拖动和大文件 Range 请求。
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func downloadZip(w http.ResponseWriter, r *http.Request, dir string, rel string) {
	zipName := filepath.Base(dir)
	if zipName == "." || zipName == string(filepath.Separator) {
		zipName = "uploads"
	}
	if !strings.HasSuffix(strings.ToLower(zipName), ".zip") {
		zipName += ".zip"
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition(zipName))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	baseInZip := path.Base(filepath.ToSlash(rel))
	if baseInZip == "." || baseInZip == "/" {
		baseInZip = "uploads"
	}

	err := filepath.WalkDir(dir, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == dir {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		innerRel, err := filepath.Rel(dir, current)
		if err != nil {
			return err
		}
		zipPath := path.Join(baseInZip, filepath.ToSlash(innerRel))
		if entry.IsDir() {
			zipPath += "/"
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipPath
		if !entry.IsDir() {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		file, err := os.Open(current)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		log.Printf("打包下载失败: %v", err)
		// 响应头已经发出，不能再可靠地返回 JSON，只记录日志。
		_ = r.Body.Close()
	}
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", http.MethodDelete)
		writeJSON(w, http.StatusMethodNotAllowed, jsonResponse{OK: false, Message: "仅支持 DELETE"})
		return
	}

	target, _, err := requestPath(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: err.Error()})
		return
	}
	if target == uploadRoot {
		writeJSON(w, http.StatusBadRequest, jsonResponse{OK: false, Message: "不能删除上传根目录"})
		return
	}

	if err := os.RemoveAll(target); err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonResponse{OK: false, Message: "删除失败"})
		return
	}
	writeJSON(w, http.StatusOK, jsonResponse{OK: true, Message: "已删除"})
}

func requestPath(r *http.Request) (string, string, error) {
	rel := r.URL.Query().Get("path")
	cleanRel, err := cleanRelativePath(rel)
	if err != nil {
		return "", "", err
	}
	target, err := secureJoin(cleanRel)
	if err != nil {
		return "", "", err
	}
	return target, cleanRel, nil
}

func cleanRelativePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimLeft(value, "/")

	if value == "" || value == "." {
		return "", errors.New("路径不能为空")
	}
	if strings.ContainsRune(value, 0) {
		return "", errors.New("路径包含非法字符")
	}
	if strings.Contains(value, ":") {
		return "", errors.New("路径不能包含冒号")
	}

	clean := path.Clean(value)
	if clean == "." || clean == "/" || clean == "" {
		return "", errors.New("路径不能为空")
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", errors.New("禁止路径穿越")
	}
	return clean, nil
}

func secureJoin(rel string) (string, error) {
	target := filepath.Join(uploadRoot, filepath.FromSlash(rel))
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	absTarget = filepath.Clean(absTarget)

	rootWithSep := uploadRoot + string(filepath.Separator)
	if absTarget != uploadRoot && !strings.HasPrefix(absTarget, rootWithSep) {
		return "", errors.New("文件路径超出 uploads 目录")
	}
	return absTarget, nil
}

func contentDisposition(filename string) string {
	escaped := url.PathEscape(filename)
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", sanitizeASCIIName(filename), escaped)
}

func inlineContentDisposition(filename string) string {
	escaped := url.PathEscape(filename)
	return fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s", sanitizeASCIIName(filename), escaped)
}

func sanitizeASCIIName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 32 && r <= 126 && r != '"' && r != '\\' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "download"
	}
	return b.String()
}

func writeJSON(w http.ResponseWriter, status int, resp jsonResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
