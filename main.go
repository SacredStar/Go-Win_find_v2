package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Signature struct {
	signature      string
	fileSignPath   string
	additionalPath string
	IsWhite        bool
}

func CloseReportFile(file *os.File) {
	_, err := file.Write([]byte(`</table></head><body></body></html>`))
	if err != nil {
		panic(err)
	}
	err = file.Close()
	if err != nil {
		panic(err)
	}
}

func AddResultToFile(file *os.File, lineNum int, lineString string, path string, signature Signature) {
	if len(lineString) > 150 {
		lineString = lineString[:150]
	}
	_, err := file.Write([]byte(`</tr>`))
	if err != nil {
		return
	}
	if _, err := file.WriteString(
		"<td>" + strconv.Itoa(lineNum) + "</td>" +
			"<td>" + path + "</td>" +
			"<td>" + signature.signature + "</td>" +
			"<td>" + lineString + "</td>" +
			"<td>" + strconv.FormatBool(signature.IsWhite) + "</td>" + "\n"); err != nil {
		log.Fatal(err)
	}
}

func FindSignatureAndReplaceIfExist(s []Signature, signToFind Signature) []Signature {
	//var isWhite bool
	for idx, v := range s {
		if signToFind.signature == v.signature {
			(s)[idx].fileSignPath = signToFind.fileSignPath
			return s
		}
	}
	s = append(s, signToFind)
	return s
}

func GetIgnoredExtension() (ignoredExt []string) {
	file, _ := os.Open("./ignore_list.txt")
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		Extension := sc.Text()
		ignoredExt = append(ignoredExt, Extension)
	}
	return ignoredExt
}

func GetWhitelist(path string) (Signs []Signature) {
	err := filepath.Walk(path, func(wPath string, info os.FileInfo, err error) error {
		// Обход директории без вывода
		if wPath == path {
			fmt.Printf("Proceeding the White Listing...\n")
			return nil
		}
		if info.IsDir() {
			fmt.Printf("[%s]\n", wPath)
			return filepath.SkipDir
		}
		if wPath != path {
			file, err := os.Open(wPath)
			if err != nil {
				return nil
			}
			sc := bufio.NewScanner(file)
			for sc.Scan() {
				sign := sc.Text()
				Signs = append(Signs, Signature{sign, "", wPath, true})
			}
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return Signs
}

func GetAllSigns(path string, Signs []Signature) []Signature {
	err := filepath.Walk(path, func(wPath string, info os.FileInfo, err error) error {
		// Обход директории без вывода
		if wPath == path {
			fmt.Printf("Proceeding All Crypto Signs...\n")
			return nil
		}
		if info.IsDir() {
			fmt.Printf("[%s]\n We skiping included folders, we assume that the Signs folder contain only txt files \n", wPath)
			return filepath.SkipDir
		}
		if wPath != path {
			file, err := os.Open(wPath)
			if err != nil {
				return nil
			}
			sc := bufio.NewScanner(file)
			for sc.Scan() {
				Signs = FindSignatureAndReplaceIfExist(Signs, Signature{sc.Text(), wPath, "", false})
			}
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return Signs
}

func ProcessSrcFiles(SrcDir string, reportDir string, signs []Signature) {
	for _, s := range signs {
		reportFile, err := ExecuteReportFile(reportDir, s)
		if err != nil {
			fmt.Printf("We can't process report file for sign: %s,path: %s, error:%s", s.signature, s.fileSignPath, err)
			continue
		}
		err = filepath.Walk(SrcDir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				//Если директория  или файлы библиотек/исполняемые - пропускаем
				if IsIgnoredExtensionOrDir(path, info) {

				} else {
					//Если файл, обрабатываем
					_, scanner := OpenToReadSrcFile(path)
					line := 1
					for scanner.Scan() {
						lineStr := scanner.Text()
						if strings.Contains(lineStr, s.signature) {
							AddResultToFile(reportFile, line, lineStr, path, s)
						}
						line++
					}
				}
				return nil
			})
		defer reportFile.Close()
		//CloseReportFile(reportFile)
	}
}

func OpenToReadSrcFile(path string) (*os.File, *bufio.Scanner) {
	fSrc, err := os.Open(path)
	if err != nil {
		log.Fatal("Error opening file: ", path, "Error: ", err)
	}
	scanner := bufio.NewScanner(fSrc)
	//Увеличиваем размер буфера, замедляет программу на ~ 30 % по бенчмаркам,
	const maxCapacity = 65536 * 300
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	return fSrc, scanner
}

func IsIgnoredExtensionOrDir(path string, fi os.FileInfo) bool {
	isIgnored := false
	for _, ext := range GetIgnoredExtension() {
		if filepath.Ext(path) == ext {
			isIgnored = true
			break
		}
	}
	if fi.IsDir() || isIgnored {
		return true
	} else {
		return false
	}
}

func ExecuteReportFile(reportDir string, s Signature) (*os.File, error) {
	//Create ReportDir if not exist
	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		if err := os.Mkdir(reportDir, 0777); err != nil {
			log.Fatal("Cant create dir:", err)
		}
	}
	//Create or get descriptor of report file in ReportDir
	if s.fileSignPath == "" {
		fmt.Printf("%v", s)
	}
	path := strings.Split(s.fileSignPath, "\\")
	//fmt.Printf("Path: %s,AdditionalPath: %v,Sign: %s\n", s.fileSignPath, s.additionalPath, s.signature)
	reportFileName := fmt.Sprint(reportDir + "\\" + path[1] + ".html")
	reportFile, err := os.OpenFile(reportFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0777)
	if err != nil {
		return nil, fmt.Errorf("error while creating report file for sign: %s, error: %s", s.signature, err)
	}
	// If size ==0 Create html template for report file
	fInfo, _ := reportFile.Stat()
	if fInfo.Size() == 0 {
		CreateHtmlTemplate(reportFile)
	}
	return reportFile, nil
}

func CreateHtmlTemplate(reportFile *os.File) {
	if _, err := reportFile.Write([]byte(`<!DOCTYPE html>
	<html lang="en">
	<head>
	<meta charset="UTF-8">
	<title>Report</title>
	<style>
       table,th,td {
           border: 1px solid grey
        }
    </style>
	<table>
	<th> №Line </th>
    <th> Path </th>
    <th>  Sign </th>
    <th>  String </th>
	<th>  isWhite? </th>`)); err != nil {
		return
	}
	if _, err := reportFile.Write([]byte(`<tr>`)); err != nil {
		panic(err)
	}
}

func main() {
	SrcDir := flag.String("SRC_DIR", "./Source", "Директория исходных файлов исследуемого ПО")
	CryptoSignsDir := flag.String("CRYPTO_SIGN_DIR", "./SignsCrypto", "Директория файлов-сигнатур ПО")
	ReportDir := flag.String("REPORT_DIR", "./Report", "Директория для формирования отчета")
	WhiteListDir := flag.String("CP5", "./CP5TestSigns", "Директория для файлов сигнатур WL")
	flag.Parse()

	Signs := GetWhitelist(*WhiteListDir)
	Signs = GetAllSigns(*CryptoSignsDir, Signs)
	ProcessSrcFiles(*SrcDir, *ReportDir, Signs)

}
