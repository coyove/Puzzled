package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"hash/crc32"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var subFlag = flag.String("sub", "", "Weibo user cookie")
var dirFlag = flag.String("dir", "", "Source images directory")
var outFlag = flag.String("outdir", "", "Output directory")
var passFlag = flag.String("pass", "", "Passord")

type imaged struct {
	Index int
	Name  string
	Path  string
	Enc   string
	Url   string
}

func UploadWeibo(fn string) (string, error) {

	buf, err := uploadWeibo(fn)
	if err != nil {
		return "", err
	}

	idx := strings.IndexByte(string(buf), '{')
	if idx >= len(buf) || idx < 0 {
		return "", errors.New("Invalid response")
	}

	tmp := make(map[string]interface{})
	json.Unmarshal(buf[idx:], &tmp)

	p := tmp["data"].(map[string]interface{})
	if p["pics"] == nil {
		return "", errors.New("Server returns error")
	}

	p = p["pics"].(map[string]interface{})
	if p["pic_1"] == nil {
		return "", errors.New("Server returns error")
	}

	p = p["pic_1"].(map[string]interface{})

	if pid, e := p["pid"]; e {
		return pid2url(pid.(string)), nil
	} else {
		return "", errors.New("Uploading failed")
	}
}

func uploadWeibo(fn string) ([]byte, error) {
	file, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("app", "miniblog")
	writer.WriteField("token", "I-Love-You")
	writer.WriteField("s", "json")
	writer.WriteField("rotate", "0")
	writer.WriteField("logo", "1")
	writer.WriteField("nick", "")
	writer.WriteField("url", "")
	writer.WriteField("cb", "")

	part, err := writer.CreateFormFile("pic1", filepath.Base(fn))
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", `http://picupload.service.weibo.com/interface/pic_upload.php?&mime=image%2Fjpeg&url=0&markpos=1&logo=&nick=0&marks=1&app=miniblog`, body)

	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("Cookie", `SUB=`+*subFlag+";")
	req.Header.Add("User-Agent", `Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36`)
	req.Header.Add("Host", "picupload.service.weibo.com")
	req.Header.Add("Origin", "http://picupload.service.weibo.com")
	req.Header.Add("Referer", "http://picupload.service.weibo.com/interface/")

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	buf, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return buf, err
}

func pid2url(pid string) string {
	var zone string

	itype := "large"

	if pid[9] == 'w' {
		zone = string((crc32.ChecksumIEEE([]byte(pid)) & 3) + 1 + '0')
		return "http://ww" + zone + ".sinaimg.cn/" + itype + "/" + pid + ".jpg"
	} else {
		return "http://ww1.sinaimg.cn/large/" + pid + ".jpg"
	}
}

func main() {
	os.Mkdir("temp", 0777)
	flag.Parse()

	files, err := ioutil.ReadDir(*dirFlag)
	if err != nil {
		log.Fatalln(err)
	}

	var pass uint64
	if *passFlag == "" {
		pass = 0xc0ffee
	} else {
		p, err := strconv.ParseInt(*passFlag, 16, 64)
		if err != nil {
			log.Fatalln(err)
		}

		pass = uint64(p)
	}

	data := make(map[string]imaged, 16)
	// data["password"] = strconv.FormatInt(int64(pass), 16)
	log.Println("**** Password is:", strconv.FormatInt(int64(pass), 16), "****")

	encChan := make(chan imaged)
	cleanExit := func() {
		log.Println("Finished")
		buf, _ := json.Marshal(data)
		ioutil.WriteFile(*dirFlag+"/.puzzle", buf, 0777)
		os.Exit(0)
	}
	go func() {
		for {
			select {
			case img, ok := <-encChan:
				if ok {
					if img.Url == "" {
						if *subFlag != "" {
							if url, err := UploadWeibo(img.Enc); err != nil {
								log.Println("ERR:", img.Name, err)
								img.Url = ""
							} else {
								log.Println(img.Name, "uploaded")
								img.Url = url
							}
						}
					} else {
						log.Println(img.Name, "already uploaded")
					}

					data[img.Name] = img
					buf, _ := json.Marshal(data)
					ioutil.WriteFile(*dirFlag+"/.puzzle", buf, 0777)
				} else {
					cleanExit()
				}
			}
		}
	}()

	cc := make(chan os.Signal, 1)
	signal.Notify(cc, os.Interrupt)
	go func() {
		for _ = range cc {
			cleanExit()
		}
	}()

	puzzled, err := ioutil.ReadFile(*dirFlag + "/.puzzle")
	if err == nil {
		json.Unmarshal(puzzled, &data)
	}

	for idx, file := range files {
		name := strings.ToLower(file.Name())
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".png") {
			fn := *dirFlag + "/" + name
			out := *outFlag + "/" + name
			if *outFlag == "" {
				out = "./temp/" + name
			}
			if strings.HasSuffix(out, ".jpg") {
				out = out[:len(out)-3] + "png"
			}

			img := imaged{}
			if data[name].Enc == "" {
				img.Path = fn
				img.Name = name
				img.Index = idx

				if err := puzzle(fn, out, pass); err != nil {
					img.Enc = ""
					log.Println("ERR:", name, err)
					data[img.Name] = img
					continue
				}

				img.Enc = out
				log.Println(name, "puzzled")
			} else {
				img = data[name]
				log.Println(name, "already puzzled")
			}

			encChan <- img
		}
	}

	close(encChan)
	for {
	}
}
