package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

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
	case "ls-tree":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "usage: mygit ls-tree <object>\n")
			os.Exit(1)
		}
		treeSha := os.Args[3]
		treePath := path.Join(".git", "objects", treeSha[:2], treeSha[2:])
		reader, err := os.Open(treePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %s\n", err)
			os.Exit(1)
		}
		zlibReader, err := zlib.NewReader(reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating zlib reader: %s\n", err)
			os.Exit(1)
		}
		decompressedContents, err := ioutil.ReadAll(zlibReader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
			os.Exit(1)
		}
		decompressedContents = decompressedContents[bytes.IndexByte(decompressedContents, 0)+1:]
		var names []string
		for len(decompressedContents) > 0 {
			mode := decompressedContents[:strings.IndexByte(string(decompressedContents), ' ')]
			decompressedContents = decompressedContents[len(mode)+1:]
			name := decompressedContents[:strings.IndexByte(string(decompressedContents), 0)]
			decompressedContents = decompressedContents[len(name)+1:]
			sha := decompressedContents[:20]
			decompressedContents = decompressedContents[len(sha):]
			names = append(names, string(name))
		}
		for _, name := range names {
			fmt.Printf("%s\n", name)
		}
	case "write-tree":
		currentDir, _ := os.Getwd()
		h, c := calcTreeHash(currentDir)
		treeHash := hex.EncodeToString(h)
		os.Mkdir(filepath.Join(".git", "objects", treeHash[:2]), 0755)
		var compressed bytes.Buffer
		w := zlib.NewWriter(&compressed)
		w.Write(c)
		w.Close()
		os.WriteFile(filepath.Join(".git", "objects", treeHash[:2], treeHash[2:]), compressed.Bytes(), 0644)
		fmt.Println(treeHash)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
func calcTreeHash(dir string) ([]byte, []byte) {
	fileInfos, _ := ioutil.ReadDir(dir)
	type entry struct {
		fileName string
		b        []byte
	}
	var entries []entry
	contentSize := 0
	for _, fileInfo := range fileInfos {
		if fileInfo.Name() == ".git" {
			continue
		}
		if !fileInfo.IsDir() {
			f, _ := os.Open(filepath.Join(dir, fileInfo.Name()))
			b, _ := ioutil.ReadAll(f)
			s := fmt.Sprintf("blob %d\u0000%s", len(b), string(b))
			sha1 := sha1.New()
			io.WriteString(sha1, s)
			s = fmt.Sprintf("100644 %s\u0000", fileInfo.Name())
			b = append([]byte(s), sha1.Sum(nil)...)
			entries = append(entries, entry{fileInfo.Name(), b})
			contentSize += len(b)
		} else {
			b, _ := calcTreeHash(filepath.Join(dir, fileInfo.Name()))
			s := fmt.Sprintf("40000 %s\u0000", fileInfo.Name())
			b2 := append([]byte(s), b...)
			entries = append(entries, entry{fileInfo.Name(), b2})
			contentSize += len(b2)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].fileName < entries[j].fileName })
	s := fmt.Sprintf("tree %d\u0000", contentSize)
	b := []byte(s)
	for _, entry := range entries {
		b = append(b, entry.b...)
	}
	sha1 := sha1.New()
	io.WriteString(sha1, string(b))
	return sha1.Sum(nil), b
}
