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
	encryptDocs = flag.Bool("e", false, "encrypt docs dir")
	openDoc     = flag.String("o", "", "open document by matching `regex`")
)

var (
	docDir = os.Getenv("HOME") + "/.local/papers"
	//	docDir     = "papers"
	keyFname   = "key"
	saltFname  = "salt"
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

func walkFn(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	relpath, err := filepath.Rel(docDir, path)
	if err != nil {
		return err
	}

	relpath = filepath.Join(filepath.Base(docDir), relpath)

	if info.IsDir() {
		relpath += "/"
	}

	header := chunkHeader{
		Path:    relpath,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
	}

	var headerChunk bytes.Buffer
	enc := gob.NewEncoder(&headerChunk)
	err = enc.Encode(header)
	if err != nil {
		return err
	}

	err = binary.Write(encrypter, binary.LittleEndian, int64(headerChunk.Len()))
	if err != nil {
		panic(err)
	}

	written, err := encrypter.Write(headerChunk.Bytes())
	if written != headerChunk.Len() {
		panic(err)
	}

	if !info.IsDir() {
		err = binary.Write(encrypter, binary.LittleEndian, info.Size())
		if err != nil {
			panic(err)
		}

		f, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		written, err := io.Copy(encrypter, f)
		if written != info.Size() {
			panic(err)
		}
	}

	totalBytes += info.Size()
	count++
	fmt.Println(count, header.Path)

	return nil
}

func prompt(keys []openpgp.Key, symmetric bool) ([]byte, error) {
	return key, nil
}

func scanFile() {

	encryptedFile, err := os.Open("papers.xdoc")
	if err != nil {
		log.Fatal(err)
	}

	md, err := openpgp.ReadMessage(encryptedFile, nil, prompt, nil)
	if err != nil {
		log.Fatal(err)
	}

	decrypter := md.LiteralData.Body

	for {
		var chunkLen int64
		var n int

		// read header size
		err = binary.Read(decrypter, binary.LittleEndian, &chunkLen)
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(err)
		}

		// read header
		buf := make([]byte, chunkLen)
		n, err = io.ReadFull(decrypter, buf)
		if int64(n) != chunkLen {
			println(n, chunkLen)
			panic(err)
		}

		headerChunk := bytes.NewReader(buf)
		dec := gob.NewDecoder(headerChunk)

		var header chunkHeader
		err = dec.Decode(&header)
		if err != nil {
			panic(err)
		}

		if header.Mode.IsDir() {
			if *openDoc == "" {
				fmt.Println("---\n" + header.Path)
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

		// read file size
		err = binary.Read(decrypter, binary.LittleEndian, &chunkLen)
		if err != nil {
			panic(err)
		}

		// open file
		if open {
			fname := "x-" + filepath.Base(header.Path)
			f, err := os.Create(fname)
			if err != nil {
				panic(err)
			}

			written, err := io.CopyN(f, decrypter, chunkLen)
			if int64(written) != chunkLen {
				println(written, chunkLen)
				panic(err)
			}
			f.Close()

			cmd := exec.Command("open", fname)
			err = cmd.Run()
			if err != nil {
				panic(err)
			}
			return
		}

		// skip file
		written, err := io.CopyN(ioutil.Discard, decrypter, chunkLen)
		if int64(written) != chunkLen {
			println(written, chunkLen)
			panic(err)
		}
	}
}

func main() {
	flag.Parse()

	if *createKey != "" {
		salt, err := generateRandomBytes(32)
		if err != nil {
			panic(err)
		}
		dk := pbkdf2.Key([]byte(*createKey), salt, 4096, 32, sha256.New)
		err = ioutil.WriteFile(saltFname, salt, 0400)
		if err != nil {
			panic(err)
		}
		str := base32.StdEncoding.EncodeToString(dk)
		err = ioutil.WriteFile(keyFname, []byte(str), 0400)
		if err != nil {
			panic(err)
		}
		return
	}

	if *restoreKey != "" {
		saltPwd := strings.Split(*restoreKey, ",")
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
		return
	}

	var err error
	key, err = ioutil.ReadFile(keyFname)
	if err != nil {
		panic(err)
	}

	if *encryptDocs {
		f, err := os.Create("papers.xdoc")
		if err != nil {
			panic(err)
		}

		hints := &openpgp.FileHints{
			IsBinary: true,
			FileName: "papers.xdoc",
			ModTime:  time.Now().UTC(),
		}

		config := &packet.Config{}
		config.DefaultCipher = packet.CipherAES256
		config.DefaultCompressionAlgo = packet.CompressionNone
		encrypter, err = openpgp.SymmetricallyEncrypt(f, key, hints, config)
		if err != nil {
			log.Fatal(err)
		}

		filepath.Walk(docDir, walkFn)
		encrypter.Close()
		f.Close()
		println("totalBytes:", totalBytes/1024/1024, "Mib")
		return
	}

	scanFile()
}
