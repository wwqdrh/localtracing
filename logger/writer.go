package logger

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	msgPool = &sync.Pool{
		New: func() interface{} {
			return &msg{tm: time.Now().In(loc)}
		},
	}

	loc      *time.Location
	fs       []*FileWriter
	lock     = &sync.Mutex{}
	warnFunc func(string)
)

const (
	defaultW1 = 1000
	defaultW2 = 2000
	defaultDn = 3000

	warnIndex1 = 0
	warnIndex2 = 1
	dropIndex  = 2
)

// FileWriter ...
type FileWriter struct {
	fileHandle io.Writer
	lastHandle *os.File

	head    *msgQue      // 队头
	tail    *msgQue      // 对尾
	size    int          // 队列长度
	in      chan *msg    // 消息入队通道
	out     chan *msg    // 消息出队通道
	refile  bool         // 等待文件切换
	dropCnt int64        // 丢弃的日志数量
	lock    *sync.Mutex  // 同步io的锁
	warn    [3]time.Time // 上次发送警告时间

	option *fileWriterOption
}

type msgQue struct {
	msg  *msg
	next *msgQue
}

type msg struct {
	tm  time.Time
	buf []byte
}

type FileOptionHandle func(*fileWriterOption)

// fileWriterOption ...
type fileWriterOption struct {
	path          string    // 日志根目录，默认目录：./log
	label         string    // 日志标签，子目录
	name          string    // 日志文件名
	clone         io.Writer // 日志克隆输出接口
	compressCount int       // 仅在按日压缩模式下有效，设置为压缩几天前的日志，支持大于等于1的数字
	compressKeep  int       // 前多少次的压缩文件删除掉。例如：1=保留最近1个压缩日志，2=保留最近2个压缩日志，依次类推。。。
	async         bool      // 异步io
	warnNumber    [3]int    // 积压数据警告数量配置
}

func init() {
	loc, _ = time.LoadLocation("Asia/Shanghai")
}

func NewOption(opts ...FileOptionHandle) *fileWriterOption {
	pc, _, _, _ := runtime.Caller(1)
	l := strings.ReplaceAll(filepath.Base(runtime.FuncForPC(pc).Name()), ".", "_")
	if l == "" {
		l = "zxlog"
	} else {
		l = strings.ToLower(l)
	}
	op := &fileWriterOption{
		path:  "./log",
		label: l,
		name:  "log",
	}
	op.warnNumber[warnIndex1] = defaultW1
	op.warnNumber[warnIndex2] = defaultW2
	op.warnNumber[dropIndex] = defaultDn

	for _, f := range opts {
		f(op)
	}

	return op
}

func WithPath(p string) FileOptionHandle {
	return func(dwo *fileWriterOption) {
		dwo.path = strings.ToLower(p)
	}
}

func WithLabel(l string) FileOptionHandle {
	return func(dwo *fileWriterOption) {
		dwo.label = strings.ToLower(l)
	}
}

func WithName(n string) FileOptionHandle {
	return func(dwo *fileWriterOption) {
		dwo.name = strings.ToLower(n)
	}
}

func WithClone(c io.Writer) FileOptionHandle {
	return func(dwo *fileWriterOption) {
		dwo.clone = c
	}
}

func WithCompress(count, keep int) FileOptionHandle {
	return func(dwo *fileWriterOption) {
		dwo.compressCount = count
		dwo.compressKeep = keep

		if dwo.compressCount < 2 {
			dwo.compressCount = 2
		}
	}
}

func WithAsync(b bool) FileOptionHandle {
	return func(fwo *fileWriterOption) {
		fwo.async = b
	}
}

func WithWarnDropNumber(w1, w2, dn int) FileOptionHandle {
	return func(fwo *fileWriterOption) {
		fwo.warnNumber[warnIndex1] = w1
		fwo.warnNumber[warnIndex2] = w2
		fwo.warnNumber[dropIndex] = dn
	}
}

// NewFileWriter ...
func NewFileWriter(option *fileWriterOption) *FileWriter {
	o := &FileWriter{option: option, head: &msgQue{}, in: make(chan *msg, 1000), out: make(chan *msg, 2), lock: &sync.Mutex{}}
	lock.Lock()
	fs = append(fs, o)
	lock.Unlock()
	if o.option == nil {
		o.option = &fileWriterOption{path: "./log"}
	}
	if o.option.path == "" {
		o.option.path = "./log"
	}
	if o.option.label != "" {
		o.option.label = "/" + o.option.label
	}
	if o.option.compressCount < 2 {
		o.option.compressCount = 2
	}
	if o.option.compressKeep < 0 {
		o.option.compressKeep = 0
	}
	o.option.compressKeep += o.option.compressCount

	if o.option.warnNumber[warnIndex1] <= 10 {
		o.option.warnNumber[warnIndex1] = defaultW1
	}
	if o.option.warnNumber[warnIndex2] <= 20 {
		o.option.warnNumber[warnIndex2] = defaultW2
	}
	if o.option.warnNumber[dropIndex] <= 30 {
		o.option.warnNumber[dropIndex] = defaultDn
	}

	o.next()

	if o.option.async {
		go o.transMsg()
		go o.dateCheck()
		go o.realIO()
	}

	go o.backend()

	return o
}

func (o *FileWriter) next() {
	f := o.getFilename(time.Now().In(loc), "log")
	os.MkdirAll(filepath.Dir(f), 0755)
	nc, err := os.OpenFile(f, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
		return
	}

	// 一分钟后关闭文件句柄
	if o.lastHandle != nil {
		oldnc := o.lastHandle
		go func(f *os.File) {
			time.Sleep(time.Minute)
			f.Close()
		}(oldnc)
	}

	// 设置新文件句柄
	o.lastHandle = nc
	if o.option.clone != nil {
		o.fileHandle = io.MultiWriter(nc, o.option.clone)
	} else {
		o.fileHandle = nc
	}
}

func (o *FileWriter) getFilename(t time.Time, suf string) string {
	if suf == "" {
		suf = "log"
	}
	return filepath.Join(o.option.path, o.option.label, t.Format("/2006/01/"), o.option.name+t.Format("_2006-01-02")+"."+suf)
}

func (o *FileWriter) backend() {
	for {
		if !o.refile {
			time.Sleep(time.Millisecond)
			continue
		}
		// 下一个日志文件
		o.next()

		o.refile = false

		// 压缩几天前的日志
		if o.option.compressCount >= 1 {
			go func() {
				t := time.Now().In(loc).Add(-time.Hour * time.Duration(24*o.option.compressCount))

				logFile := o.getFilename(t, "log")
				zipFile := o.getFilename(t, "zip")
				if err := compressAndRemoveFile(logFile, zipFile); err != nil {
					log.Println(err)
				}

				// 删除过期日志
				if o.option.compressKeep > 0 {
					t := time.Now().In(loc).Add(-time.Hour * time.Duration(24*(o.option.compressKeep+1)))
					zipFile := o.getFilename(t, "zip")
					if err := os.RemoveAll(zipFile); err != nil {
						log.Println(err)
					}
				}
			}()
		}
	}
}

// 队列只在这里操作，不用加锁
func (o *FileWriter) transMsg() {
	last := time.Now().In(loc)
	warnMsg := func(index int) {
		now := time.Now()
		if now.Sub(o.warn[index]) > time.Hour {
			msg := fmt.Sprintf("文件[%s/%s/%s] 日志缓存量[%d]达到警告级别%d", o.option.path, o.option.label, o.option.name, o.size, index+1)
			if index == dropIndex {
				msg += fmt.Sprintf(", 已丢弃[%d]", o.dropCnt)
			} else {
				msg += fmt.Sprintf("[%d]", o.option.warnNumber[index])
			}

			if warnFunc != nil {
				warnFunc(msg)
			} else {
				log.Println(msg)
			}
			o.warn[index] = now
		}
	}
	for {
		select {
		case msg := <-o.in:
			if o.tail == nil {
				o.tail = o.head
			}
			o.tail.next = &msgQue{msg: msg}
			o.tail = o.tail.next
			if len(msg.buf) == 0 {
				continue
			}
			o.size++
			if o.size > o.option.warnNumber[dropIndex] {
				warnMsg(dropIndex)
				msg := o.head.next
				if msg == nil {
					continue
				}
				if msg.msg.tm.Day() != last.Day() {
					last = msg.msg.tm
					o.refile = true
				}

				o.head.next = msg.next
				if o.head.next == nil {
					o.tail = o.head
				}
				if len(msg.msg.buf) > 0 {
					o.size--
					atomic.AddInt64(&o.dropCnt, 1)
					// 如果文件io过载，则直接输出到屏幕
					log.Println(string(msg.msg.buf))
				}
			} else if o.size > o.option.warnNumber[warnIndex2] {
				warnMsg(warnIndex2)
			} else if o.size > o.option.warnNumber[warnIndex1] {
				warnMsg(warnIndex1)
			}
		default:
			if o.refile {
				continue
			}

			msg := o.head.next
			if msg == nil {
				continue
			}

			if msg.msg.tm.Day() != last.Day() {
				last = msg.msg.tm
				o.refile = true
				continue
			}

			if len(msg.msg.buf) == 0 {
				o.head.next = msg.next
				if o.head.next == nil {
					o.tail = o.head
				}
				continue
			}

			select {
			case o.out <- msg.msg:
				o.head.next = msg.next
				if o.head.next == nil {
					o.tail = o.head
				}
				o.size--
			default:
			}
		}
	}
}

func (o *FileWriter) realIO() {
	for msg := range o.out {
		if msg == nil || len(msg.buf) == 0 {
			continue
		}

		o.fileHandle.Write(msg.buf)
	}
}

func (o *FileWriter) dateCheck() {
	for {
		// 等待明天
		t1 := time.Now().In(loc)
		t2, _ := time.ParseInLocation("2006-01-02", t1.Add(time.Hour*24).Format("2006-01-02"), t1.Location())
		<-time.After(t2.Sub(t1))
		if o.option.async {
			o.in <- &msg{tm: time.Now().In(loc)}
		} else {
			o.refile = true
		}
	}
}

func (o *FileWriter) Write(p []byte) (n int, err error) {
	if o.fileHandle == nil {
		return 0, errors.New("io nil error")
	}
	if o.option.async {
		msg := msgPool.Get().(*msg)
		msg.tm = time.Now().In(loc)
		msg.buf = msg.buf[:0]
		msg.buf = append(msg.buf, p...)
		select {
		case o.in <- msg:
			return len(p), nil
		default:
			atomic.AddInt64(&o.dropCnt, 1)
			return len(p), nil
		}
	}

	o.lock.Lock()
	defer o.lock.Unlock()
	return o.fileHandle.Write(p)
}

func compressAndRemoveFile(file, zipFile string) error {
	fz, err := os.Create(zipFile)
	if err != nil {
		return err
	}
	defer fz.Close()

	w := zip.NewWriter(fz)
	defer w.Close()

	fDest, err := w.Create(filepath.Base(file))
	if err != nil {
		return err
	}
	fSrc, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fSrc.Close()
	_, err = io.Copy(fDest, fSrc)
	if err != nil {
		return err
	}

	// 删除日志文件
	return os.RemoveAll(file)
}

func msgRemaining() int {
	cnt := 0
	for _, f := range fs {
		cnt += f.size
	}
	return cnt
}

func msgDroped() int64 {
	var cnt int64
	for _, f := range fs {
		cnt += f.dropCnt
	}
	return cnt
}
