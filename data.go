package main

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/juntaki/transparent/s3"
	"github.com/k0kubun/pp"
)

type s3Bare interface {
	getBare() (key s3.BareKey, value *s3.Bare, err error)
	setBare(b *s3.Bare) error
}

func (w *Wikidata) saveBare(item s3Bare) error {
	stack := w.cacheStack[reflect.TypeOf(item).Elem()]

	bareKey, bareValue, err := item.getBare()
	if err != nil {
		return err
	}

	bareKey.Bucket = w.bucket
	return stack.Set(bareKey, bareValue)
}

func (w *Wikidata) loadBare(item s3Bare) error {
	stack := w.cacheStack[reflect.TypeOf(item).Elem()]

	bareKey, _, err := item.getBare()
	if err != nil {
		return err
	}
	bareKey.Bucket = w.bucket
	bareValue, err := stack.Get(bareKey)
	if err != nil {
		return err
	}

	err = item.setBare(bareValue.(*s3.Bare))
	if err != nil {
		return err
	}
	return nil
}

func (w *Wikidata) deleteBare(item s3Bare) error {
	stack := w.cacheStack[reflect.TypeOf(item).Elem()]
	bareKey, _, err := item.getBare()
	if err != nil {
		return err
	}
	bareKey.Bucket = w.bucket
	err = stack.Remove(bareKey)
	if err != nil {
		return err
	}
	return nil
}

type pageData struct {
	titleHash  string // Key
	versionId  string // Key
	title      string
	author     string
	body       string
	lastUpdate time.Time
}

func (page *pageData) getBare() (key s3.BareKey, value *s3.Bare, err error) {
	bk := s3.BareKey{
		Key:       "page/" + page.titleHash + "/index.md",
		VersionId: page.versionId,
	}

	bv := s3.NewBare()
	bv.Value["Body"] = []byte(page.body)
	bv.Value["ContentType"] = aws.String("text/x-markdown")
	bv.Value["Metadata"] = map[string]*string{
		"Author": aws.String(page.author),
		"Title":  aws.String(base64.StdEncoding.EncodeToString([]byte(page.title))),
	}
	return bk, bv, nil
}

func (page *pageData) setBare(b *s3.Bare) error {
	page.lastUpdate = *b.Value["LastModified"].(*time.Time)

	body, err := readAndSeek(b.Value["Body"])
	if err != nil {
		return err
	}
	page.body = string(body)

	meta := b.Value["Metadata"].(map[string]*string)
	decode, err := base64.StdEncoding.DecodeString(*meta["Title"])
	if err != nil {
		return err
	}
	page.title = string(decode)
	//page.id = *meta["ID"]
	page.author = *meta["Author"]
	return nil
}

type htmlData struct {
	titleHash string // Key
	body      string
}

func (h *htmlData) getBare() (key s3.BareKey, value *s3.Bare, err error) {
	bk := s3.BareKey{
		Key: "page/" + h.titleHash + "/index.html",
	}

	bv := s3.NewBare()
	bv.Value["Body"] = strings.NewReader(h.body)
	bv.Value["ContentType"] = aws.String("text/html")
	return bk, bv, nil
}

func (h *htmlData) setBare(b *s3.Bare) error {
	panic("don't load html")
}

type userData struct {
	ID               string `json:"id"` // Key
	Name             string `json:"name"`
	AuthenticateType string `json:"authtype"`
	Token            string `json:"token"`
	Secret           string `json:"secret"`
}

func (user *userData) getBare() (key s3.BareKey, value *s3.Bare, err error) {
	bk := s3.BareKey{
		Key: "user/" + user.Name,
	}

	body, err := json.Marshal(user)
	if err != nil {
		return bk, nil, err
	}
	bv := s3.NewBare()
	bv.Value["Body"] = body
	bv.Value["ContentType"] = aws.String("text/plain")
	return bk, bv, nil
}

func (user *userData) setBare(b *s3.Bare) error {
	body, err := readAndSeek(b.Value["Body"])
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, user)
	if err != nil {
		return err
	}
	return nil
}

type fileData struct {
	filename    string // Key
	titleHash   string // Key
	filebyte    []byte
	contentType string
	acl         string
}

func (file *fileData) getBare() (key s3.BareKey, value *s3.Bare, err error) {
	bk := s3.BareKey{
		Key: "page/" + file.titleHash + "/file/" + file.filename,
	}

	bv := s3.NewBare()
	bv.Value["Body"] = file.filebyte
	bv.Value["ContentType"] = aws.String(file.contentType)
	bv.Value["ACL"] = file.acl
	return bk, bv, nil
}

func (file *fileData) setBare(b *s3.Bare) error {
	body, err := readAndSeek(b.Value["Body"])
	if err != nil {
		return err
	}

	file.filebyte = body
	file.contentType = *b.Value["ContentType"].(*string)
	//file.acl = *b.Value["ACL"].(*string)
	return nil
}

type sessionData struct {
	ID         string `json:"id"` // Key
	Challange  string `json:"challange"`
	User       string `json:"user"`
	BreadCrumb []byte `json:"breadcrumb"`
	Login      bool   `json:"login"`
}

func (session *sessionData) getBare() (key s3.BareKey, value *s3.Bare, err error) {
	bk := s3.BareKey{
		Key: "session/" + session.ID,
	}

	body, err := json.Marshal(session)
	if err != nil {
		pp.Print(session)
		return bk, nil, err
	}
	bv := s3.NewBare()
	bv.Value["Body"] = body
	bv.Value["ContentType"] = aws.String("text/plain")
	return bk, bv, nil
}

func (session *sessionData) setBare(b *s3.Bare) error {
	body, err := readAndSeek(b.Value["Body"])
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, session)
	if err != nil {
		pp.Print(body)
		return err
	}
	return nil
}

func readAndSeek(value interface{}) ([]byte, error) {
	// reader, ok := value.(io.Reader)
	// if !ok {
	// 	return []byte{}, fmt.Errorf("type is %s, not io.Reader", reflect.TypeOf(value))
	// }
	// body, err := ioutil.ReadAll(reader)
	// if err != nil {
	// 	return []byte{}, err
	// }
	// //	value = body

	return value.([]byte), nil
}
