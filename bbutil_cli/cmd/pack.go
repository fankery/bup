/*
Copyright © 2023 LiuHailong
*/
package cmd

import (
	"archive/tar"
	"bbutil_cli/common"
	"bufio"
	"compress/gzip"
	"container/list"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var wg sync.WaitGroup
var copyWg sync.WaitGroup
var branchCode int

// 获取可执行文件的路径
var dir, _ = os.Executable()
var absPath, _ = filepath.Abs(filepath.Dir(dir))
var path = strings.ReplaceAll(absPath, `\`, `/`)

// packCmd represents the pack command
var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Get front-end package",
	Long:  `Obtain the front-end compressed package, decompress it, and paste it in the corresponding path`,
	Run: func(cmd *cobra.Command, args []string) {
		startTime := time.Now()
		defer func() {
			common.Logger.Infof("耗时:%d ms", time.Since(startTime).Milliseconds())
		}()
		//分支
		// 初始化一个空的切片，用于存储map
		branchMapSlice := make([]map[string]string, 0)
		branchInfo := viper.Get("branchList")

		branchList, ok := branchInfo.([]any)
		if !ok {
			common.Logger.Fatal("获取前端包配置错误")
		}
		for _, v := range branchList {
			branch, ok := v.(map[string]any)
			if !ok {
				common.Logger.Fatal("获取前端包配置错误")
			}

			code, ok := branch["code"].(int)
			if !ok {
				common.Logger.Fatal("获取前端包配置错误")
			}
			if branchCode != 0 {
				//选定分支信息
				if branchCode == code {
					branchMap := make(map[string]string, 0)
					version := fmt.Sprintf("%v", branch["version"])
					if version == "" {
						common.Logger.Fatal("请检查版本信息")
					}

					branchMap["code"] = fmt.Sprintf("%d", code)
					branchMap["version"] = version
					branchMap["srcPath"] = fmt.Sprintf("%v", branch["srcpath"])
					branchMap["targetPath"] = fmt.Sprintf("%v", branch["targetpath"])
					branchMap["prefixPath"] = fmt.Sprintf("%v", branch["prefixpath"])
					branchMapSlice = append(branchMapSlice, branchMap)
					break
				} else {
					continue
				}
			} else {
				//全部分支信息
				branchMap := make(map[string]string, 0)
				version := fmt.Sprintf("%v", branch["version"])
				if version == "" {
					common.Logger.Fatal("请检查版本信息")
				}
				branchMap["code"] = fmt.Sprintf("%d", code)
				branchMap["version"] = version
				branchMap["srcPath"] = fmt.Sprintf("%v", branch["srcpath"])
				branchMap["targetPath"] = fmt.Sprintf("%v", branch["targetpath"])
				branchMap["prefixPath"] = fmt.Sprintf("%v", branch["prefixpath"])
				branchMapSlice = append(branchMapSlice, branchMap)
			}
		}
		if len(branchMapSlice) == 0 {
			common.Logger.Fatal("未获取到分支信息")
		}
		rules := viper.Get("rule")
		ruleSlice, ok := rules.([]any)
		if !ok {
			common.Logger.Fatal("获取前端包配置错误")
		}
		wg.Add(len(ruleSlice))
		for _, rule := range ruleSlice {
			ruleMap, ok := rule.(map[string]any)
			if !ok {
				common.Logger.Fatal("获取前端包配置错误")
			}
			root := fmt.Sprintf("%v", ruleMap["root"])
			prefix := fmt.Sprintf("%v", ruleMap["prefix"])
			url := fmt.Sprintf("http://172.31.3.252:8082/%s/?C=M;O=D", root)
			getUrl := fmt.Sprintf("http://172.31.3.252:8082/%s", root)
			ruleList, ok := ruleMap["model"].([]any)
			if !ok {
				common.Logger.Fatal("获取前端包配置错误")
			}
			//按规则分组，看需要解析几个网页
			go collyGetFront(prefix, url, getUrl, &branchMapSlice, &ruleList)
		}
		wg.Wait()
	},
}

func init() {
	packCmd.Flags().IntVarP(&branchCode, "branch", "b", 0, `branch code`)
	rootCmd.AddCommand(packCmd)
}

func collyGetFront(prefix, url, getUrl string, branchList *[]map[string]string, ruleSlice *[]any) {
	// 存储前1000条数据
	var urlList = list.New()
	c := colly.NewCollector()

	//请求前调用
	c.OnRequest(func(r *colly.Request) {
		common.Logger.Info("开始发起请求")
	})

	//请求发生错误调用
	c.OnError(func(r *colly.Response, err error) {
		common.Logger.Fatal("请求发生错误", err)
	})

	// 响应调用
	c.OnResponse(func(r *colly.Response) {
		common.Logger.Info("请求返回响应数据,开始解析")
	})

	// 解析html
	c.OnHTML("a[href]", func(h *colly.HTMLElement) {
		if urlList.Len() < 1000 {
			urlList.PushBack(h.Text)
		} else {
			return
		}
	})

	//scraped
	c.OnScraped(func(r *colly.Response) {
		common.Logger.Info("开始匹配`rule`")
		// executeWg.Add(len(*branchList))
		for _, v := range *branchList {
			ruleList := list.New()
			for _, r := range *ruleSlice {
				ruleList.PushBack(fmt.Sprintf("%v-v%v", r, v["version"]))
			}
			downloadTempDir, err := os.MkdirTemp(path, "download")
			if err != nil {
				common.Logger.Fatal("创建文件夹 download 错误", err)
			}
			//创建对应分支的临时目录
			branchTempDir, err := os.MkdirTemp(downloadTempDir, fmt.Sprintf("branch_%s", v["code"]))
			//程序退出时，删除临时目录
			if err != nil {
				common.Logger.Fatal("创建文件夹 download 错误")
			}
			analysisUrl(urlList, ruleList, fmt.Sprintf("%v", v["code"]), prefix, getUrl, branchTempDir, downloadTempDir)
			copyWg.Add(2)
			go func() {
				if err := copyDir(absPath+"/download/branch_"+fmt.Sprintf("%v", v["code"])+fmt.Sprintf("%v", v["prefixPath"]), fmt.Sprintf("%v", v["srcPath"])); err != nil {
					common.Logger.Fatal("粘贴失败", err)
				}
				copyWg.Done()
			}()
			go func() {
				if err := copyDir(absPath+"/download/branch_"+fmt.Sprintf("%v", v["code"])+fmt.Sprintf("%v", v["prefixPath"]), fmt.Sprintf("%v", v["targetPath"])); err != nil {
					common.Logger.Fatal("粘贴失败", err)
				}
				copyWg.Done()
			}()
			copyWg.Wait()
			common.Logger.Info("粘贴成功")
		}
		wg.Done()
	})

	if err := c.Visit(url); err != nil {
		common.Logger.Fatal(err)
	}
}

// 解析
func analysisUrl(urlList *list.List, ruleList *list.List, code, prefix, url, branchTempDir, downloadTempDir string) {
	defer func() {
		branchTempDir, _ = filepath.Abs(branchTempDir)
		downloadTempDir, _ = filepath.Abs(downloadTempDir)
		if err := os.RemoveAll(branchTempDir); err != nil {
			common.Logger.Fatal("删除临时目录错误")
		}
		if err := os.RemoveAll(downloadTempDir); err != nil {
			common.Logger.Fatal("删除临时目录错误")
		}
	}()
	//url 大循环
	for i := urlList.Front(); i != nil; i = i.Next() {
		//匹配规则小循环
		for j := ruleList.Front(); j != nil; j = j.Next() {
			//匹配的正则表达式
			regStr := fmt.Sprintf("^%s-%s.*tar.gz$", prefix, j.Value.(string))
			//regStr := "^qince-" + j.Value.(string) + ".*tar\\.gz$"
			//编译正则
			compile, err := regexp.Compile(regStr)
			if err != nil {
				common.Logger.Fatal("正则编译失败")
			}
			matched := compile.MatchString(i.Value.(string))
			if matched {
				common.Logger.Debug("["+code+"] 匹配到： ", i.Value)
				// wg.Add(1)
				download(branchTempDir+"/"+i.Value.(string), url+"/"+i.Value.(string), code)
				ruleList.Remove(j)
			}
		}
	}
	// executeWg.Done()
}

func download(filePath string, downloadUrl string, code string) {
	filePathAbs, _ := filepath.Abs(filePath)
	// http: //172.31.3.252:8082/qince/qince-web-dms-v7.0.5-20220801094854.tar.gz
	f, err := os.Create(filePathAbs)
	if err != nil {
		common.Logger.Fatal("文件创建失败", err)
	}
	wt := bufio.NewWriter(f)
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			common.Logger.Fatal("文件创建失败", err)
		}
	}(f)

	resp, err := http.Get(downloadUrl)
	if err != nil {
		common.Logger.Fatal("[err] 下载文件错误", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			common.Logger.Fatal("[err] 下载文件错误", err)
		}
	}(resp.Body)

	_, err = io.Copy(wt, resp.Body)
	if err != nil {
		common.Logger.Fatal(err)
	}
	err = wt.Flush()
	if err != nil {
		return
	}
	unTarGZ(filePathAbs, code)
	// wg.Done()
}

// 解压
func unTarGZ(filepath string, code string) {
	open, err := os.Open(filepath)
	if err != nil {
		common.Logger.Fatal("文件解压失败", err)
	}
	defer func(open *os.File) {
		err := open.Close()
		if err != nil {
			return
		}
	}(open)
	//	解压tar.gz
	gzf, err := gzip.NewReader(open)
	if err != nil {
		common.Logger.Fatal("文件解压失败", err)
	}
	defer func(reader *gzip.Reader) {
		err := reader.Close()
		if err != nil {
			return
		}
	}(gzf)

	tr := tar.NewReader(gzf)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			common.Logger.Fatal("文件解压失败", err)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			_, err := os.Stat(path + "/download/branch_" + code + "/" + header.Name)
			if err == nil {
				continue
			} else {
				err := os.MkdirAll(path+"/download/branch_"+code+"/"+header.Name, 0755)
				if err != nil {
					common.Logger.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
				}
			}
		case tar.TypeReg:
			outFile, err := os.Create(path + "/download/branch_" + code + "/" + header.Name)
			if err != nil {
				common.Logger.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				common.Logger.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
		default:
			common.Logger.Fatalf(header.Name)
		}
	}
}

func copyFile(srcPath, destPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

func copyDir(srcDir, destDir string) error {
	// 遍历源目录
	return filepath.Walk(srcDir, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 构建目标路径
		destPath := filepath.Join(destDir, srcPath[len(srcDir):])

		if info.IsDir() {
			// 如果是目录，创建目标目录
			return os.MkdirAll(destPath, os.ModePerm)
		} else if info.Mode().IsRegular() {
			// 如果是文件，复制文件内容到目标文件，如果目标文件已存在则替换
			if err := copyFile(srcPath, destPath); err != nil {
				return err
			}
		}
		return nil
	})
}
