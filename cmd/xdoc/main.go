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
	createKey   = flag.String("ck", "", "create encryption key protected by `password`")
	restoreKey  = flag.String("rk", "", "restore encryption key using `salt_file,password`")

	printConfig = flag.Bool("g", false, "print effective config")
	encryptDocs = flag.Bool("c", false, "encrypt docs dir into xdoc file")
	openDoc     = flag.String("o", "", "open document from xdoc by matching `regex`")
	listDocs = flag.Bool("t", false, "list all entries in xdoc file")
	extractDocs = flag.Bool("x", false, "extract xdoc file info docs dir")

	docsDir = flag.String("docs", os.Getenv("HOME") + "/.local/papers", "set docs dir")
	xdocFile = flag.String("xdoc", os.Getenv("HOME") + "/.local/xdoc/papers.xdoc", "set xdoc file")
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

type chunkHeader struct {
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
}

func writeHeaderChunk(w io.Writer, info os.FileInfo, relpath string) {
	header := chunkHeader{
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

func readHeaderChunk(r io.Reader) (chunkHeader, error) {
	var chunkLen int64
	var n int

	// read header size
	err := binary.Read(r, binary.LittleEndian, &chunkLen)
	if err != nil {
		return chunkHeader{}, err
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

	var header chunkHeader
	err = dec.Decode(&header)
	if err != nil {
		panic(err)
	}

	return header, nil
}

func readFileChunk(r io.Reader, dst string) {
	var chunkLen int64

	err := binary.Read(r, binary.LittleEndian, &chunkLen)
	if err != nil {
		panic(err)
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
}

func scanXdocFile() {

	encryptedFile, err := os.Open(*xdocFile)
	if err != nil {
		log.Fatal(err)
	}

	md, err := openpgp.ReadMessage(encryptedFile, nil, prompt, nil)
	if err != nil {
		log.Fatal(err)
	}

	decrypter := md.LiteralData.Body

	for {
		header, err := readHeaderChunk(decrypter)
		if err == io.EOF {
			break
		}

		if header.Mode.IsDir() {
			if *openDoc == "" {
				fmt.Println("---\n" + header.Path)
			}

			if *extractDocs {
				dirpath := filepath.Join(filepath.Dir(*docsDir), header.Path)
				fmt.Println("creating dir", dirpath)
				if err = os.MkdirAll(dirpath, 0700); err != nil {
					panic(err)
				}
			}
			continue
		}

		open := false

		if *openDoc != "" {
			if strings.Contains(header.Path, *openDoc) {
				fmt.Println(header.Path)
				open = true
			}
		} else {
			fmt.Println(header.Path)
		}

		dst := ""
		if open {
			dst = filepath.Join(os.TempDir(), "xdoc-" + filepath.Base(header.Path))
		} else if *extractDocs {
			dst = filepath.Join(filepath.Dir(*docsDir), header.Path)
			fmt.Println("extracting to", dst)
		}

		readFileChunk(decrypter, dst)

		if open {
			cmd := exec.Command("xpdf", dst)
			err = cmd.Run()
			if err != nil {
				panic(err)
			}
			return
		}
	}
}

func createSymmetricKey() {
	salt, err := generateRandomBytes(32)
	if err != nil {
		panic(err)
	}
	dk := pbkdf2.Key([]byte(*createKey), salt, 4096, 32, sha256.New)
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
	defer encrypter.Close()
	defer f.Close()

	err = filepath.Walk(*docsDir, walkFn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("totalBytes:", totalBytes/1024/1024, "Mib")
}

func printEffectiveConfig() {
	fmt.Println("docsDir:", *docsDir)
	fmt.Println("xdocFile:", *xdocFile)
	fmt.Println("keyFname:", keyFname)
	fmt.Println("saltFname:", saltFname)
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

	var err error
	key, err = ioutil.ReadFile(keyFname)
	if err != nil {
		panic(err)
	}

	if *encryptDocs {
		encryptDocsDir()
		return
	}

	if *listDocs || *openDoc != "" || *extractDocs {
		scanXdocFile()
		return
	}

	flag.Usage()
}
