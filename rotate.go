package localtracing

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RotateLogWriteSyncer
// 支持按每天，文件大小切换日志的writersyncer
// 需要使用zapcore.Lock，将RotateLogWriteSyncer加锁，支持协程安全
type RotateLogWriteSyncer struct {
	when            string
	rotateSize      int64
	currentFileNo   int
	maxFileNo       int
	currentFileSize int64
	filePath        string
	logWriter       *bufio.Writer
	logFile         *os.File
	nextRotateTime  int64
}

func parsePath(path string) (absPath, dir, base string, err error) {
	absPath, err = filepath.Abs(path)
	if err != nil {
		return "", "", "", errors.New("logFileName's abs fail! err = " + err.Error())
	}

	dir = filepath.Dir(absPath)
	base = filepath.Base(absPath)
	err = nil

	return
}

func getCurrentLogNumber(dir, filename string) (fileSize int64, maxNumber int) {
	fileSize = 0
	maxNumber = 0

	files, _ := ioutil.ReadDir(dir)
	for _, file := range files {
		// 目录，跳过
		if file.IsDir() {
			continue
		}

		// 日志文件名相同，说明是当前正在写的文件
		if file.Name() == filename {
			fileSize = file.Size()
		}

		// 获取文件名最后的number后缀
		if strings.HasPrefix(file.Name(), filename) {
			no, err := strconv.Atoi(filepath.Ext(file.Name()))
			if err != nil {
				continue
			}

			if no > maxNumber {
				maxNumber = no
			}
		}
	}

	return
}

// 获取明天0点0分0秒的unix时间值
func getNextRotateTime() int64 {
	timeStr := time.Now().Format("2006-01-02")
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", timeStr+" 23:59:59", time.Local)
	return t.Unix() + 1
}

func createLogWriter(filePath string) (*os.File, *bufio.Writer, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, nil, err
	}

	return f, bufio.NewWriter(f), nil
}

func getLogWriter(filePath string) (*os.File, *bufio.Writer, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_RDWR, 0755)
	if err != nil {
		return nil, nil, err
	}

	return f, bufio.NewWriter(f), nil
}

//RotateLoggerInit 日志writer初始化
// when: 目前支持“MIDNIGHT”，每天0点切换日志
// rotateSize: 日志文件大小，单位M
// logFileName: 日志文件名
// maxFileNumber: 每天日志最多保存的文件个数，包括当前正在写的日志文件，最多1000个，序号从000 -- 999
func (rw *RotateLogWriteSyncer) RotateLoggerInit(when string, rotateSize int64, logFileName string, maxFileNumberPerDay int) error {
	if when != "MIDNIGHT" {
		return errors.New("currently only supports 'MIDNIGHT'")
	} else {
		rw.when = when
	}

	if logFileName == "" {
		return errors.New("logFileName is missing")
	}

	// 解析日志存放目录，文件名
	var dirPath, fileName string
	var err error
	rw.filePath, dirPath, fileName, err = parsePath(logFileName)
	if err != nil {
		return err
	}

	// 获取日志序号
	rw.currentFileSize, rw.currentFileNo = getCurrentLogNumber(dirPath, fileName)
	rw.currentFileNo++

	// 计算下一次日志切换时间
	rw.nextRotateTime = getNextRotateTime()

	// 日志大小限制
	if rotateSize <= 0 {
		// 不限制大小
		rw.rotateSize = 0xFFFFFFFF
	} else {
		rw.rotateSize = rotateSize * 1024 * 1024
	}

	// 保留的日志文件个数
	if (maxFileNumberPerDay <= 0) || (maxFileNumberPerDay > 1000) {
		rw.maxFileNo = 1000
	} else {
		rw.maxFileNo = maxFileNumberPerDay
	}

	// 做一次日志切换，避免生成tmp文件后，程序异常退出，存在tmp文件的情况
	doLogRotate(rw.filePath, rw.maxFileNo, time.Now().Format("2006-01-02"))

	// 如果filesize为0，表示还没有日志，新创建，否则打开原有文件
	if rw.currentFileSize == 0 {
		rw.logFile, rw.logWriter, err = createLogWriter(rw.filePath)
		if err != nil {
			return err
		}
	} else {
		rw.logFile, rw.logWriter, err = getLogWriter(rw.filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rw *RotateLogWriteSyncer) isNeedRotate(strLen int64) bool {
	// 时间判断
	if time.Now().Unix() >= rw.nextRotateTime {
		rw.nextRotateTime = getNextRotateTime()
		return true
	}

	// 文件大小判断
	if (strLen + rw.currentFileSize) > rw.rotateSize {
		return true
	}

	return false
}

func getRotateFileNameList(filePath string, maxFileNo int, dateStr string) (rotateFileList, deleteFileList []string, extNos []int) {
	_, dir, base, err := parsePath(filePath)
	if err != nil {
		return
	}

	files, _ := ioutil.ReadDir(dir)
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if file.Name() == base {
			continue
		}

		if file.Name() == (base + ".tmp") {
			continue
		}

		if strings.HasPrefix(file.Name(), base+"."+dateStr) {
			no, err := strconv.Atoi(filepath.Ext(file.Name())[1:])
			if err != nil {
				continue
			}

			// maxFileno数减去当前正在写的日志文件数1，编号从000开始，需要再减1
			if no >= (maxFileNo - 2) {
				// 文件后缀超过最大编号，删除该文件
				deleteFileList = append(deleteFileList, dir+"/"+file.Name())
			} else {
				extNos = append(extNos, no)
				rotateFileList = append(rotateFileList, dir+"/"+file.Name())
			}
		}
	}

	return
}

func doLogRotate(filePath string, maxFileNo int, dateStr string) {
	rotateFileList, deleteFileList, extNos := getRotateFileNameList(filePath, maxFileNo, dateStr)

	if len(deleteFileList) > 0 {
		for i := 0; i < len(deleteFileList); i++ {
			os.Remove(deleteFileList[i])
		}
	}

	// 待改名文件后缀
	if (len(extNos) > 0) && (len(rotateFileList) > 0) {
		sort.Ints(extNos)
		for idx := len(extNos); idx > 0; idx-- {
			dstFileName := rotateFileList[idx-1][:len(rotateFileList[idx-1])-3] + fmt.Sprintf("%03d", extNos[idx-1]+1)
			os.Rename(rotateFileList[idx-1], dstFileName)
		}
	}

	// 最后将tmp文件改名为000
	os.Rename(filePath+".tmp", filePath+"."+dateStr+".000")
}

func (rw *RotateLogWriteSyncer) Write(p []byte) (n int, err error) {
	// 判断时间，文件大小条件，是否需要切换日志,如果需要切换日志，执行日志切换动作
	if rw.isNeedRotate(int64(len(p))) {
		// 切换日志
		// 先将当前文件重命名，新建日志文件
		rw.logWriter.Flush()
		rw.logFile.Close()
		os.Rename(rw.filePath, rw.filePath+".tmp")

		rw.logFile, rw.logWriter = nil, nil
		rw.logFile, rw.logWriter, err = createLogWriter(rw.filePath)
		rw.currentFileSize = 0

		// 然后另起协程，重命名旧日志文件
		doLogRotate(rw.filePath, rw.maxFileNo, time.Now().Format("2006-01-02"))
	}

	// 写入日志
	if rw.logFile == nil {
		rw.logFile, rw.logWriter, err = createLogWriter(rw.filePath)
		if err == nil {
			n, err = rw.logWriter.Write(p)
			rw.currentFileSize += int64(len(p))
		}
	} else {
		n, err = rw.logWriter.Write(p)
		rw.currentFileSize += int64(n)
	}
	return
}

func (rw *RotateLogWriteSyncer) Sync() error {
	return rw.logWriter.Flush()
}
