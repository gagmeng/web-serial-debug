package main

import (
	"context"
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/webview/webview_go"
	"window/bridge"
)

// 这些变量会在编译时通过 -ldflags 注入
// go build -ldflags "-X main.buildTime=$(date) -X main.version=1.0.0"
var (
	buildTime = "unknown"
	version   = "dev"
)

// GetVersionInfo 返回版本信息
func getVersionInfo() map[string]string {
	return map[string]string{
		"buildTime": buildTime,
		"version":   version,
	}
}

func missingFrontendPage(indexPath string, err error) string {
	return "data:text/html," + url.PathEscape(fmt.Sprintf(`
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>Serial Debug Tool</title>
    <style>
      body { font-family: Arial, sans-serif; padding: 24px; line-height: 1.6; }
      code, pre { background: #f5f5f5; padding: 2px 4px; border-radius: 4px; }
      pre { padding: 12px; white-space: pre-wrap; }
    </style>
  </head>
  <body>
    <h2>Frontend resources were not found</h2>
    <p>The desktop app expected <code>dist/index.html</code> next to the executable.</p>
    <pre>Path: %s
Error: %s</pre>
    <p>Please package the <code>dist</code> folder together with the executable, and build the frontend with <code>npm run build:desktop</code> or <code>yarn build:desktop</code>.</p>
  </body>
</html>`,
		html.EscapeString(indexPath), html.EscapeString(err.Error())))
}

func startStaticServer(distDir string) (string, func(), error) {
	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return "", nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}

	server := &http.Server{
		Handler: http.FileServer(http.Dir(distDir)),
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Static server stopped unexpectedly: %v", err)
		}
	}()

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			log.Printf("Failed to shut down static server: %v", err)
		}
	}

	return fmt.Sprintf("http://%s", listener.Addr().String()), shutdown, nil
}

func main() {
	debug := flag.Bool("debug", false, "Enable debug mode")
	useInfluxDB := flag.Bool("use-influxdb", false, "Use InfluxDB instead of embedded database")
	influxDBURL := flag.String("influxdb-url", "http://localhost:8086", "InfluxDB server URL")
	influxDBToken := flag.String("influxdb-token", "", "InfluxDB authentication token")
	influxDBOrg := flag.String("influxdb-org", "myorg", "InfluxDB organization")
	influxDBBucket := flag.String("influxdb-bucket", "mybucket", "InfluxDB bucket")
	flag.Parse()

	// 初始化配置
	config := bridge.NewConfig()
	config.UseInfluxDB = *useInfluxDB
	config.InfluxDBURL = *influxDBURL
	config.InfluxDBToken = *influxDBToken
	config.InfluxDBOrg = *influxDBOrg
	config.InfluxDBBucket = *influxDBBucket

	// 如果使用InfluxDB，初始化客户端连接
	var influxClient influxdb2.Client
	if config.UseInfluxDB {
		influxClient = influxdb2.NewClient(config.InfluxDBURL, config.InfluxDBToken)
		defer influxClient.Close()

		// 测试连接
		health, err := influxClient.Health(context.Background())
		if err != nil {
			log.Printf("Warning: Failed to connect to InfluxDB: %v", err)
		} else {
			log.Printf("Connected to InfluxDB %s", health.Version)
		}
	}

	// 创建webview实例
	w := webview.New(*debug)
	defer w.Destroy()

	// 设置窗口标题和大小
	w.SetTitle("Serial Debug Tool")
	w.SetSize(1024, 768, webview.HintNone)

	// 创建bridge实例，传入数据库配置
	b := bridge.New(w)
	b.SetConfig(config)
	if config.UseInfluxDB {
		b.SetInfluxClient(influxClient)
	}

	// 注册串口相关的JavaScript桥接函数
	w.Bind("initSerial", b.InitSerial)
	w.Bind("writeSerial", b.WriteSerial)
	w.Bind("readSerial", b.ReadSerial)

	// 注册文件操作相关的JavaScript桥接函数
	w.Bind("saveFile", b.SaveFile)
	w.Bind("readFile", b.ReadFile)
	w.Bind("listDirectory", b.ListDirectory)

	// 注册数据存储相关的JavaScript桥接函数
	w.Bind("saveDataPoint", b.SaveDataPoint)
	w.Bind("queryData", b.QueryData)

	// 注册版本信息相关的JavaScript桥接函数
	w.Bind("getVersionInfo", getVersionInfo)

	// 获取应用资源路径
	var resourcePath string
	var stopStaticServer func()
	if *debug {
		resourcePath = "http://localhost:5173"
	} else {
		exePath, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		resourceDir := filepath.Dir(exePath)
		if runtime.GOOS == "darwin" {
			resourceDir = filepath.Join(resourceDir, "../Resources")
		}
		distDir := filepath.Join(resourceDir, "dist")
		resourcePath2, shutdown, err := startStaticServer(distDir)
		if err != nil {
			indexPath := filepath.Join(distDir, "index.html")
			log.Printf("Failed to locate frontend entry: %s (%v)", indexPath, err)
			resourcePath = missingFrontendPage(indexPath, err)
		} else {
			resourcePath = resourcePath2
			stopStaticServer = shutdown
			defer stopStaticServer()
			log.Printf("Loading frontend from %s", resourcePath)
		}
	}

	// 加载前端页面
	w.Navigate(resourcePath)

	// 运行主循环
	w.Run()
}
