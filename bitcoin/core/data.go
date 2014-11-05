package core

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type DB struct {
	Path string

	Files map[int]*os.File
	Id    int

	BufferedWriter *bufio.Writer

	Position int64
}

const WriteBufferSize = 20 * 1000 * 1000
const MaxFileSize = 4 * 1000 * 1000 * 1000

func (db *DB) OpenFile(id int) *os.File {
	path := filepath.Join(db.Path, fmt.Sprintf("part%d", id))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		log.Panic("error opening file: %v\n", err)
	}

	return file
}

func (db *DB) SwitchFile() {
	if db.BufferedWriter != nil {
		if err := db.BufferedWriter.Flush(); err != nil {
			log.Panic(err)
		}
	}

	db.Id++
	file := db.OpenFile(db.Id)
	db.BufferedWriter = bufio.NewWriterSize(file, WriteBufferSize)
	db.Files[db.Id] = file
	db.Position = 0
}

func CreateDB(path string) *DB {
	os.MkdirAll(path, 0770)

	files, _ := ioutil.ReadDir(path)
	for _, file := range files {
		log.Printf("deleting %v\n", file.Name())
		os.Remove(filepath.Join(path, file.Name()))
	}

	db := &DB{
		Path:     path,
		Files:    make(map[int]*os.File),
		Id:       -1,
		Position: MaxFileSize,
	}

	return db
}

func ContinueDB(path string, token int64) *DB {
	length := int(token >> 40)
	id := int(token>>32) & ((1 << 8) - 1)
	offset := token & ((1 << 32) - 1)

	db := &DB{
		Path:     path,
		Files:    make(map[int]*os.File),
		Id:       id,
		Position: offset + int64(length),
	}

	for i := 0; i <= id; i++ {
		db.Files[i] = db.OpenFile(i)
	}

	db.Files[id].Seek(db.Position, os.SEEK_SET)
	db.BufferedWriter = bufio.NewWriterSize(db.Files[id], WriteBufferSize)

	return db
}

func (db *DB) Write(data []byte) int64 {
	if db.Position >= MaxFileSize {
		db.SwitchFile()
	}

	if len(data) >= 1<<24 {
		log.Panic("too big")
	}

	if n, err := db.BufferedWriter.Write(data); n != len(data) || err != nil {
		log.Panic(err)
	}

	token := (int64(len(data)) << 40) | (int64(db.Id) << 32) | db.Position
	db.Position += int64(len(data))

	return token
}

func (db *DB) Read(token int64) []byte {
	length := int(token >> 40)
	id := int(token>>32) & ((1 << 8) - 1)
	offset := token & ((1 << 32) - 1)

	if id == db.Id && offset+int64(length) >= db.Position-int64(db.BufferedWriter.Buffered()) {
		if err := db.BufferedWriter.Flush(); err != nil {
			log.Panic(err)
		}
	}

	file := db.Files[id]

	buffer := make([]byte, length)
	n, err := file.ReadAt(buffer[:], offset)
	if n != length || err != nil {
		log.Panic(err)
	}

	return buffer
}
