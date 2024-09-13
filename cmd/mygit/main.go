package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}
	catfile := flag.NewFlagSet("cat-file", flag.ExitOnError)
	hashobj := flag.NewFlagSet("hash-object", flag.ExitOnError)
	lstree := flag.NewFlagSet("ls-tree", flag.ExitOnError)
	catp := catfile.Bool("p", true, "print type of content")
	// hashT := hashobj.String("t", "blob", "The type of content")
	nameOnly := lstree.Bool("name-only", true, "get only the file name")
	hashW := hashobj.Bool("w", true, "save to the git objects")
	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}
		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}
		fmt.Println("Initialized git directory")
	case "cat-file":
		catfile.Parse(os.Args[2:])
		sha1Hash := strings.Join(catfile.Args(), "")
		dir, file := sha1Hash[:2], sha1Hash[2:]
		if _, err := os.Stat(".git/objects/" + dir); !os.IsNotExist(err) {
			if content, err := os.ReadFile(".git/objects/" + dir + "/" + file); err == nil {
				decompressor, _ := zlib.NewReader(bytes.NewBuffer(content))
				con := make([]byte, 1024)
				decompressor.Read(con)
				content_split := strings.Split(string(con), "\x00")
				if *catp {
					fmt.Print(content_split[1])
				} else {
					fmt.Print(content_split[0])
				}
			} else {
				panic(err)
			}
		} else {
			panic(err)
		}
	case "hash-object":
		hashobj.Parse(os.Args[2:])
		filename := hashobj.Args()
		file_content, err := os.ReadFile(strings.Join(filename, ""))
		if err != nil {
			panic(err)
		}
		var sb strings.Builder
		sb.WriteString("blob ")
		sb.WriteString(strconv.Itoa(len(file_content)))
		sb.WriteString("\x00")
		sb.WriteString(string(file_content))
		hasher := sha1.New()
		hasher.Write([]byte(sb.String()))
		hash := hasher.Sum(nil)
		if *hashW {
			dir := fmt.Sprintf("%x", hash[:1])
			file := fmt.Sprintf("%x", hash[1:])
			os.MkdirAll(".git/objects/"+dir, 0755)
			openr, _ := os.Create(".git/objects/" + dir + "/" + file)
			zlibWriter := zlib.NewWriter(openr)
			zlibWriter.Write([]byte(sb.String()))
			zlibWriter.Close()
			fmt.Print(fmt.Sprintf("%x", hash))
		} else {
			fmt.Print(fmt.Sprintf("%x", hash))
		}
	case "ls-tree":
		lstree.Parse(os.Args[2:])
		restArgs := strings.Join(lstree.Args(), "")
		dir, file := restArgs[:2], restArgs[2:]
		filePath := filepath.Join(".git/objects", dir, file)
		openedFile, _ := os.Open(filePath)
		zlibReader, _ := zlib.NewReader(openedFile)
		con, _ := io.ReadAll(zlibReader)
		if *nameOnly {
			split := bytes.Split(con, []byte("\x00"))
			use := split[1 : len(split)-1]
			for _, dByte := range use {
				splitter := []byte(" ")
				splitByte := bytes.Split(dByte, splitter)[1]
				fmt.Println(string(splitByte))
			}
		} else {
			// Todo parse whole tree
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
