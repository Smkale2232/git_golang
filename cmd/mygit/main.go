package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

// Initialize a git repo
func init_repo() {
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
}

// Read a blob identified by its hash and return its contents
func read_object(blob_sha string) []byte {
	path := fmt.Sprintf(".git/objects/%s/%s", blob_sha[:2], blob_sha[2:])
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Failed to open blob file: %s\n", err)
	}
	reader, err := zlib.NewReader(file)
	defer reader.Close()
	if err != nil {
		log.Fatalf("Failed to instantitate zlib reader: %s\n", err)
	}
	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, reader)
	if err != nil {
		log.Fatalf("Failed to write to stdout: %s\n", err)
	}
	// Object type
	_, err = buffer.ReadBytes(byte(' '))
	if err != nil {
		log.Fatalf("Failed to read from buffer: %s\n", err)
	}
	// Object size
	size_byte, err := buffer.ReadBytes(byte(0))
	if err != nil {
		log.Fatalf("Failed to read from buffer: %s\n", err)
	}
	size, err := strconv.Atoi(string(size_byte[:len(size_byte)-1]))
	if err != nil {
		log.Fatalf("Failed to convert number of bytes into integer: %s\n", err)
	}
	buffer.Truncate(size)
	return buffer.Bytes()
}

// Create an object from the given byte array and return the object's SHA
func create_object(content []byte) []byte {
	// SHA1 hash writer
	hash_writer := sha1.New()
	// zlib writer
	var blob_content_buffer bytes.Buffer
	zlib_writer := zlib.NewWriter(&blob_content_buffer)
	// Write blob content to both writers
	writer := io.MultiWriter(hash_writer, zlib_writer)
	writer.Write(content)
	// Retrieve SHA1 hash
	sha := hash_writer.Sum(nil)
	sha_string := fmt.Sprintf("%x", sha)
	// Flush compressed blob content
	zlib_writer.Close()
	// Create blob
	blob_dir := fmt.Sprintf(".git/objects/%s", sha_string[:2])
	err := os.MkdirAll(blob_dir, 0755)
	if err != nil {
		log.Fatalf("Failed to create directory for object: %s\n", err)
	}
	blob_path := fmt.Sprintf("%s/%s", blob_dir, sha_string[2:])
	err = os.WriteFile(blob_path, blob_content_buffer.Bytes(), 0644)
	if err != nil {
		log.Fatalf("Failed to write blob to file: %s\n", err)
	}
	return sha
}

// Read a file identified by its path, create object, and return blob sha
func hash_file(path string) []byte {
	f, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read from given file: %s\n", err)
	}
	// Create content
	content := []byte(fmt.Sprintf("blob %d\x00", len(f)))
	content = append(content, f...)
	return create_object(content)
}

// Read a tree object identified by its hash and return entries, which in turn specify blobs
func read_tree(hash string) {
	file, err := os.Open(fmt.Sprintf(".git/objects/%s/%s", hash[:2], hash[2:]))
	if err != nil {
		log.Fatalf("Failed to open tree object file: %s\n", err)
	}
	reader, err := zlib.NewReader(file)
	defer reader.Close()
	if err != nil {
		log.Fatalf("Failed to instantitate zlib reader: %s\n", err)
	}
	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, reader)
	if err != nil {
		log.Fatalf("Failed to write to stdout: %s\n", err)
	}
	// Object type
	_, err = buffer.ReadBytes(byte(' '))
	if err != nil {
		log.Fatalf("Failed to read from buffer: %s\n", err)
	}
	// Object size
	size_byte, err := buffer.ReadBytes(byte(0))
	_, err = strconv.Atoi(string(size_byte[:len(size_byte)-1]))
	if err != nil {
		log.Fatalf("Failed to convert tree object size to integer: %s\n", err)
	}
	// Read each entry
	sha_buffer := make([]byte, 20)
	for {
		_, err = buffer.ReadBytes(byte(' '))
		if err != nil {
			log.Fatalf("Failed to read from buffer first: %s\n", err)
		}
		name, err := buffer.ReadBytes(byte(0))
		if err != nil {
			log.Fatalf("Failed to read from buffer second: %s\n", err)
		}
		fmt.Println(string(name[:len(name)-1]))
		_, err = io.ReadFull(&buffer, sha_buffer)
		if err != nil {
			log.Fatalf("Failed to read 20 bytes from buffer: %s\n", err)
		}
		if buffer.Len() == 0 {
			break
		}
	}
}

// Read a directory identified by its path, create a tree object, and return object sha
func hash_tree(dir string) []byte {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("Failed to list directory: %s\n", err)
	}
	// Generate tree object entries_buffer
	var entries_buffer bytes.Buffer
	for _, entry := range entries {
		name := entry.Name()
		path := fmt.Sprintf("%s/%s", dir, name)
		if name == ".git" {
			continue
		}
		var sha []byte
		var mode string
		if entry.IsDir() {
			mode = "40000"
			sha = hash_tree(path)
		} else {
			mode = "100644"
			sha = hash_file(path)
		}
		_, err = entries_buffer.Write([]byte(fmt.Sprintf("%s %s\x00", mode, name)))
		if err != nil {
			log.Fatalf("Failed to write to byte buffer: %s\n", err)
		}
		_, err = entries_buffer.Write(sha)
		if err != nil {
			log.Fatalf("Failed to write to byte buffer: %s\n", err)
		}
	}
	content := []byte(fmt.Sprintf("tree %d\x00", entries_buffer.Len()))
	content = append(content, entries_buffer.Bytes()...)
	return create_object(content)
}

// Create a commit object given info
func commit_tree(tree_sha, parent_sha, message string) []byte {
	commit_content := []byte(fmt.Sprintf(
		"tree %s\nparent %s\nauthor Jae-Won Chung <jwnchung@umich.edu> 1702733092 +0900\ncommitter Jae-Won Chung <jwnchung@umich.edu> 1702733092 +0900\n\n%s\n",
		tree_sha,
		parent_sha,
		message),
	)
	content := []byte(fmt.Sprintf("commit %d\x00", len(commit_content)))
	content = append(content, commit_content...)
	return create_object(content)
}

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}
	switch command := os.Args[1]; command {
	case "init":
		init_repo()
	case "cat-file":
		content := read_object(os.Args[3])
		fmt.Print(string(content))
	case "hash-object":
		sha := hash_file(os.Args[3])
		fmt.Printf("%x\n", sha)
	case "ls-tree":
		read_tree(os.Args[3])
	case "write-tree":
		sha := hash_tree(".")
		fmt.Printf("%x\n", sha)
	case "commit-tree":
		sha := commit_tree(os.Args[2], os.Args[4], os.Args[6])
		fmt.Printf("%x\n", sha)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
