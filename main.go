package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"syscall"
)

var (
	lock      sync.Mutex
	lockFDMap = map[string]*sync.Mutex{}
)

func main() {
	createTestData()
	Read("./junk.bin")
	data := []byte("1234567890")
	err := Write("./junk.bin", data)
	if err != nil {
		fmt.Println(err)
	}
	Read("./junk.bin")

	Delete("./junk.bin", 20, 50)
	Read("./junk.bin")

}

func createTestData()  {
	outf,_ := os.Create("junk.bin")
	defer outf.Close()
	for i := 0; i < 100; i++ {
		var ii uint8 = uint8(i)
		err := binary.Write(outf, binary.LittleEndian, ii)
		if err != nil {
			fmt.Println("err!",err)
		}
	}
}

func Read(filename string) {
	file, err := os.OpenFile(filename, os.O_RDONLY, os.FileMode(os.ModeExclusive))
	if err != nil {
		panic(err)
	}
	defer file.Close()
	data, _ := ioutil.ReadAll(file)
	fmt.Printf("value: %v size: %d \n", data, len(data))
}

func Write(filename string, data interface{}) error {
	file, err := os.OpenFile(filename, os.O_RDWR, os.FileMode(os.ModeExclusive))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = binary.Write(file, binary.LittleEndian, data)
	if err != nil {
		return err
	}
	return nil
}

func Delete(filename string, index, offset int64) error {
	file, err := os.OpenFile(filename, os.O_RDWR, os.FileMode(os.ModeExclusive))
	if err != nil {
		return err
	}
	defer file.Close()

	// lock .Dir
	fd := file.Fd()
	err = GoFlock(fd, filename)
	if err != nil {
		return err
	}
	defer func() { _ = GoFunlock(fd, filename) }()

	// cut data By copy & re-slice
	data, _ := ioutil.ReadAll(file)
	copy(data[index:], data[index+offset:])
	data = data[:len(data)-int(offset)]

	// 1. rename old .Dir to .Dir.tmp
	err = os.Rename(filename, filename+".tmp")
	if err != nil {
		return err
	}
	// 2. create new .Dir
	newFile,_ := os.Create(filename)
	if err != nil {
		return err
	}
	defer newFile.Close()
	// 3. write new .Dir
	err = binary.Write(newFile, binary.LittleEndian, data)
	if err != nil {
		return err
	}

	// 4. remove tmp
	_ = os.Remove(filename+".tmp")

	//TODO if error, recovery to original data

	return nil
}

func GoFlock(fd uintptr, filename string) (err error) {
	err = lockFD(filename)
	if err != nil {
		return err
	}

	return syscall.Flock(int(fd), syscall.LOCK_EX)
}

func GoFunlock(fd uintptr, filename string) (err error) {
	defer func() { _ = unlockFD(filename) }()

	return syscall.Flock(int(fd), syscall.LOCK_UN)
}

func lockFD(filenameOffset string) (err error) {
	lock.Lock()
	defer lock.Unlock()

	_, ok := lockFDMap[filenameOffset]
	if ok {
		return errors.New("ErrPttLock")
	}

	theLock := &sync.Mutex{}

	theLock.Lock()
	lockFDMap[filenameOffset] = theLock

	return nil
}

func unlockFD(filenameOffset string) (err error) {
	lock.Lock()
	defer lock.Unlock()

	theLock, ok := lockFDMap[filenameOffset]
	if !ok {
		return nil
	}

	theLock.Unlock()
	delete(lockFDMap, filenameOffset)

	return nil
}
