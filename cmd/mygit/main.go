package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {
	// fmt.Println("Logs from your program will appear here!")
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}
	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}
		headFileContents := []byte("ref: refs/heads/master\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}
		fmt.Println("Initialized git directory")
	case "cat-file":
		if len(os.Args) == 4 && os.Args[2] == "-p" {
			sha := os.Args[3]
			dir := sha[0:2]
			filename := sha[2:]
			path := filepath.Join(".git", "objects", dir, filename)
			b, err := ioutil.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in cat-file p command %s\n", err)
				os.Exit(1)
			}
			buf := bytes.NewBuffer(b)
			r, err := zlib.NewReader(buf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in cat-file p command zlib %s\n", err)
				os.Exit(1)
			}
			defer r.Close()
			bs, err := io.ReadAll(r)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in cat-file p command find null char %s\n", err)
				os.Exit(1)
			}
			found := false
			for _, b := range bs {
				if found {
					fmt.Print(string(b))
				} else {
					found = b == 0
				}
			}
		}
	case "hash-object":
		file, _ := os.ReadFile(os.Args[3])
		stats, _ := os.Stat(os.Args[3])
		content := string(file)
		contentAndHeader := fmt.Sprintf("blob %d\x00%s", stats.Size(), content)
		sha := (sha1.Sum([]byte(contentAndHeader)))
		hash := fmt.Sprintf("%x", sha)
		blobName := []rune(hash)
		blobPath := ".git/objects/"
		for i, v := range blobName {
			blobPath += string(v)
			if i == 1 {
				blobPath += "/"
			}
		}
		var buffer bytes.Buffer
		z := zlib.NewWriter(&buffer)
		z.Write([]byte(contentAndHeader))
		z.Close()
		os.MkdirAll(filepath.Dir(blobPath), os.ModePerm)
		f, _ := os.Create(blobPath)
		defer f.Close()
		f.Write(buffer.Bytes())
		fmt.Print(hash)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
