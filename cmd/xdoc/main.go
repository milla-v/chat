package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
	"golang.org/x/crypto/pbkdf2"
)

var (
	createKey  = flag.String("key-create", "", "create encryption key protected by `password`")
	restoreKey = flag.String("key-restore", "", "restore encryption key using `salt_file,password`")
	checkKey  = flag.String("key-check", "", "check encryption key protected by `password`")

	printConfig = flag.Bool("g", false, "print effective config")
	encryptDocs = flag.Bool("c", false, "encrypt docs dir into xdoc file")
	createIndex = flag.Bool("index", false, "create index from xdoc file")
	readIndex = flag.Bool("index-read", false, "dump xdoc index")

	editPasswords = flag.Bool("e", false, "edit password file")
	findPassword  = flag.Bool("p", false, "find password")

	docsDir   = flag.String("docs", os.Getenv("HOME")+"/.local/papers/2017", "set docs dir")
	xdocFile  = flag.String("xdoc", os.Getenv("HOME")+"/.local/xdoc/papers-2017.xdoc", "set xdoc file")
	xindexFile = flag.String("xindex", os.Getenv("HOME")+"/.local/xdoc/index-2017.xdoc", "set xindex file")
	xpassFile = flag.String("xpass", os.Getenv("HOME")+"/.local/xdoc/pass.xdoc", "set xpass file")
)

var (
	keyFname   = os.Getenv("HOME") + "/.config/xdoc/key"
	saltFname  = os.Getenv("HOME") + "/.config/xdoc/salt"
	key        []byte
	count      int
	totalBytes int64
	encrypter  io.WriteCloser
)

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type Header struct {
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
}

func writeHeaderChunk(w io.Writer, info os.FileInfo, relpath string) {
	header := Header{
		Path:    relpath,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
	}

	var headerChunk bytes.Buffer
	enc := gob.NewEncoder(&headerChunk)
	err := enc.Encode(header)
	if err != nil {
		panic(err)
	}

	err = binary.Write(w, binary.LittleEndian, int64(headerChunk.Len()))
	if err != nil {
		panic(err)
	}

	written, err := w.Write(headerChunk.Bytes())
	if written != headerChunk.Len() {
		panic(err)
	}
}

func writeFileChunk(w io.Writer, size int64, path string) {
	err := binary.Write(w, binary.LittleEndian, size)
	if err != nil {
		panic(err)
	}

	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	written, err := io.Copy(w, f)
	if written != size {
		panic(err)
	}
}

func walkFn(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	relpath, err := filepath.Rel(*docsDir, path)
	if err != nil {
		return err
	}

	relpath = filepath.Join(filepath.Base(*docsDir), relpath)

	if info.IsDir() {
		relpath += "/"
	}

	writeHeaderChunk(encrypter, info, relpath)

	if !info.IsDir() {
		writeFileChunk(encrypter, info.Size(), path)
	}

	totalBytes += info.Size()
	count++
	fmt.Println(count, relpath)

	return nil
}

func prompt(keys []openpgp.Key, symmetric bool) ([]byte, error) {
	return key, nil
}

func readHeaderChunk(r io.Reader) (Header, error) {
	var chunkLen int64
	var n int

	// read header size
	err := binary.Read(r, binary.LittleEndian, &chunkLen)
	if err != nil {
		return Header{}, err
	}

	// read header
	buf := make([]byte, chunkLen)
	n, err = io.ReadFull(r, buf)
	if int64(n) != chunkLen {
		println(n, chunkLen)
		panic(err)
	}

	br := bytes.NewReader(buf)
	dec := gob.NewDecoder(br)

	var header Header
	err = dec.Decode(&header)
	if err != nil {
		panic(err)
	}

	return header, nil
}

func readFileChunk(r io.Reader, dst string) error {
	var chunkLen int64

	err := binary.Read(r, binary.LittleEndian, &chunkLen)
	if err != nil {
		return err
	}

	outf := ioutil.Discard

	if dst != "" {
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		outf = f
	}

	written, err := io.CopyN(outf, r, chunkLen)
	if int64(written) != chunkLen {
		println(written, chunkLen)
		panic(err)
	}

	return nil
}

type DecrypterReader struct {
	f *os.File
	decrypter io.Reader
}

func NewDecrypterReader(fname string) *DecrypterReader {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}

	md, err := openpgp.ReadMessage(f, nil, prompt, nil)
	if err != nil {
		log.Fatal(err)
	}

	return &DecrypterReader{
		f: f,
		decrypter: md.LiteralData.Body,
	}
}

func (xr *DecrypterReader) Close() error {
	return xr.f.Close()
}

func (dr *DecrypterReader) Read(p []byte) (n int, err error) {
	return dr.decrypter.Read(p)
}


type Reader struct {
	f *os.File
	decrypter io.Reader
	header *Header // current header
	chunkWasRead bool // file chunk was read
}

func NewReader(fname string) *Reader {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}

	md, err := openpgp.ReadMessage(f, nil, prompt, nil)
	if err != nil {
		log.Fatal(err)
	}

	return &Reader{
		f: f,
		decrypter: md.LiteralData.Body,
	}
}

func (xr *Reader) Next() (*Header, error) {
	if xr.header != nil && !xr.chunkWasRead && !xr.header.Mode.IsDir() {
		err := xr.SaveFile("") // skip file chunk
		if err != nil {
			return nil, err
		}
	}

	header, err := readHeaderChunk(xr.decrypter)
	if err == io.EOF {
		return nil, err
	}
	xr.chunkWasRead = false
	xr.header = &header
	return &header, nil
}

func (xr *Reader) SaveFile(dst string) error {
	if xr.chunkWasRead {
		return fmt.Errorf("already read")
	}
	err := readFileChunk(xr.decrypter, dst)
	if err != nil {
		return err
	}
	xr.chunkWasRead = true
	return nil
}

func (xr *Reader) Close() error {
	return xr.f.Close()
}

type EncrypterWriter struct {
	f *os.File
	encrypter io.WriteCloser
}

func NewEncrypterWriter(fname string) *EncrypterWriter {
	f, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}

	hints := &openpgp.FileHints{
		IsBinary: true,
		FileName: fname,
		ModTime:  time.Now().UTC(),
	}

	config := &packet.Config{}
	config.DefaultCipher = packet.CipherAES256
	config.DefaultCompressionAlgo = packet.CompressionNone
	encrypter, err = openpgp.SymmetricallyEncrypt(f, key, hints, config)
	if err != nil {
		log.Fatal(err)
	}

	return &EncrypterWriter {
		f: f,
		encrypter: encrypter,
	}
}

func (ew *EncrypterWriter) Close() error {
	ew.encrypter.Close()
	return ew.f.Close()
}

func (ew *EncrypterWriter) Write(p []byte) (n int, err error) {
	return ew.encrypter.Write(p)
}

func createXdocIndex() {
	log.Println("INFO", "create index", *xindexFile)
	ew := NewEncrypterWriter(*xindexFile)
	xr := NewReader(*xdocFile)
	defer xr.Close()
	defer ew.Close()

	enc := gob.NewEncoder(ew)

	for {
		header, err := xr.Next()
		if err == io.EOF {
			break
		}
		if err  != nil {
			log.Fatal(err)
		}
		err = enc.Encode(header)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(header.Path)
	}
}

func readXdocIndex() {
	dr := NewDecrypterReader(*xindexFile)
	defer dr.Close()

	dec := gob.NewDecoder(dr)

	for {
		var header Header
		err := dec.Decode(&header)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(header.Path)
	}
}

func createSymmetricKey() {
	salt, err := generateRandomBytes(64)
	if err != nil {
		panic(err)
	}
	dk := pbkdf2.Key([]byte(*createKey), salt, 8192, 64, sha256.New)
	err = ioutil.WriteFile(saltFname, salt, 0400)
	if err != nil {
		panic(err)
	}
	fmt.Println("generated salt saved to", saltFname)
	str := base32.StdEncoding.EncodeToString(dk)
	err = ioutil.WriteFile(keyFname, []byte(str), 0400)
	if err != nil {
		panic(err)
	}
	fmt.Println("generated key saved to", keyFname)
}

func restoreSymmetricKey(saltAndPassword string) {
	saltPwd := strings.Split(saltAndPassword, ",")
	saltFile := saltPwd[0]
	salt, err := ioutil.ReadFile(saltFile)
	if err != nil {
		panic(err)
	}
	pwd := saltPwd[1]
	dk := pbkdf2.Key([]byte(pwd), salt, 4096, 32, sha256.New)
	str := base32.StdEncoding.EncodeToString(dk)
	err = ioutil.WriteFile(keyFname, []byte(str), 0400)
	if err != nil {
		panic(err)
	}
	fmt.Println("restored key saved to", keyFname)
}

func checkSymmetricKey(password string) {
	salt, err := ioutil.ReadFile(saltFname)
	if err != nil {
		panic(err)
	}

	keybytes, err := ioutil.ReadFile(keyFname)
	if err != nil {
		panic(err)
	}

	key := string(keybytes)
	dk := pbkdf2.Key([]byte(password), salt, 4096, 32, sha256.New)
	str := base32.StdEncoding.EncodeToString(dk)

	if key == str {
		fmt.Println("key is ok")
	} else {
		fmt.Println("invalid password")
	}
}

func encryptDocsDir() {
	f, err := os.Create(*xdocFile)
	if err != nil {
		panic(err)
	}

	hints := &openpgp.FileHints{
		IsBinary: true,
		FileName: *xdocFile,
		ModTime:  time.Now().UTC(),
	}

	config := &packet.Config{}
	config.DefaultCipher = packet.CipherAES256
	config.DefaultCompressionAlgo = packet.CompressionNone
	encrypter, err = openpgp.SymmetricallyEncrypt(f, key, hints, config)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()
	defer encrypter.Close()

	err = filepath.Walk(*docsDir, walkFn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("totalBytes:", totalBytes/1024/1024, "Mib")
}

func printEffectiveConfig() {
	fmt.Println("docsDir:", *docsDir)
	fmt.Println("xdocFile:", *xdocFile)
	fmt.Println("xpassFile:", *xpassFile)
	fmt.Println("keyFname:", keyFname)
	fmt.Println("saltFname:", saltFname)
}

func encryptFile(src, dst string) error {
	fsrc, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer fsrc.Close()

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	hints := &openpgp.FileHints{
		IsBinary: true,
		FileName: filepath.Base(dst),
		ModTime:  time.Now().UTC(),
	}

	config := &packet.Config{}
	config.DefaultCipher = packet.CipherAES256
	config.DefaultCompressionAlgo = packet.CompressionNone
	encrypter, err = openpgp.SymmetricallyEncrypt(f, key, hints, config)
	if err != nil {
		return err
	}

	defer encrypter.Close()
	_, err = io.Copy(encrypter, fsrc)
	if err != nil {
		return err
	}
	return nil
}

func calcHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%x\n", h.Sum(nil))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func runPasswordEditor() {
	tmpfname := filepath.Join(os.TempDir(), "xdoc-pass.txt")

	_, err := os.Stat(tmpfname)
	if err == nil || !os.IsNotExist(err) {
		fmt.Println(tmpfname, "exists. Removing.")
		err := os.Remove(tmpfname)
		if err != nil {
			panic(err)
		}
	}

	tmpf, err := os.OpenFile(tmpfname, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}

	_, err = os.Stat(*xpassFile)

	if os.IsNotExist(err) {
		fmt.Fprintln(tmpf, "# password file\n")
	} else {
		encryptedFile, err := os.Open(*xpassFile)
		if err != nil {
			panic(err)
		}

		md, err := openpgp.ReadMessage(encryptedFile, nil, prompt, nil)
		if err != nil {
			panic(err)
		}

		decrypter := md.LiteralData.Body
		_, err = io.Copy(tmpf, decrypter)
		if err != nil {
			panic(err)
		}

		encryptedFile.Close()
	}
	tmpf.Close()

	hash1 := calcHash(tmpfname)
	cmd := exec.Command("vim", tmpfname)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	fmt.Println("vim stopped")

	hash2 := calcHash(tmpfname)
	if hash2 == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		fmt.Println("discard changes")
		return
	}

	if hash1 == hash2 {
		fmt.Println("no changes")
		return
	}

	newfname := *xpassFile + ".new"
	err = encryptFile(tmpfname, newfname)
	err2 := os.Remove(tmpfname)

	if err != nil || err2 != nil {
		os.Remove(newfname)
		if err2 != nil {
			panic(err2)
		} else {
			panic(err)
		}
	}
	err = os.Rename(*xpassFile, *xpassFile+time.Now().UTC().Format("-20060102-150405"))
	if err != nil {
		fmt.Println(err)
	}

	err = os.Rename(newfname, *xpassFile)
	if err != nil {
		panic(err)
	}
}


func main() {
	flag.Parse()

	if *printConfig {
		printEffectiveConfig()
		return
	}

	if *createKey != "" {
		createSymmetricKey()
		return
	}

	if *restoreKey != "" {
		restoreSymmetricKey(*restoreKey)
		return
	}

	if *checkKey != "" {
		checkSymmetricKey(*checkKey)
		return
	}

	var err error
	key, err = ioutil.ReadFile(keyFname)
	if err != nil {
		panic(err)
	}

	if *editPasswords {
		runPasswordEditor()
		return
	}

	if *encryptDocs {
		encryptDocsDir()
		return
	}

	if *createIndex {
		createXdocIndex()
		return
	}

	if *readIndex {
		readXdocIndex()
		return
	}

	flag.Usage()
}
