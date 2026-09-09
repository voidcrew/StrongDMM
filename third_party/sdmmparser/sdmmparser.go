package sdmmparser

/*
#cgo CFLAGS: -I./lib
#cgo LDFLAGS: -L./src/target/release -lsdmmparser
#cgo windows LDFLAGS: -lbcrypt -lntdll
#cgo linux LDFLAGS: -ldl -lm
#include <stdlib.h>
#include "lib/sdmmparser.h"
*/
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

type ObjectTreeType struct {
	Location Location
	Path     string
	Vars     []ObjectTreeVar
	Children []ObjectTreeType
}

// PlanetNoise samples the same normalized Perlin field as rust-g 4.0.
func PlanetNoise(seed uint32, x, y, step float64, width, height int) []float64 {
	if width < 1 || height < 1 || width > 1024 || height > 1024 {
		return nil
	}
	result := make([]float64, width*height)
	C.SdmmPlanetNoise(C.uint(seed), C.double(x), C.double(y), C.double(step), C.uint(width), C.uint(height), (*C.double)(unsafe.Pointer(&result[0])))
	return result
}

type Location struct {
	File   string
	Line   uint32
	Column uint16
}

type ObjectTreeVar struct {
	Name     string
	Value    string
	Decl     bool
	IsTmp    bool `json:"is_tmp"`
	IsConst  bool `json:"is_const"`
	IsStatic bool `json:"is_static"`
}

type parserError struct {
	msg string
}

func (e *parserError) Error() string {
	return e.msg
}

func IsParserError(err error) bool {
	var e *parserError
	ok := errors.As(err, &e)
	return ok
}

func ParseEnvironment(environmentPath string) (*ObjectTreeType, error) {
	return parseEnvironment(environmentPath, false)
}

func ParsePreviewEnvironment(environmentPath string) (*ObjectTreeType, error) {
	return parseEnvironment(environmentPath, true)
}

func parseEnvironment(environmentPath string, preview bool) (*ObjectTreeType, error) {
	nativePath := C.CString(environmentPath)
	defer C.free(unsafe.Pointer(nativePath))

	var nativeStr *C.char
	if preview {
		nativeStr = C.SdmmParsePreviewEnvironment(nativePath)
	} else {
		nativeStr = C.SdmmParseEnvironment(nativePath)
	}
	defer C.SdmmFreeStr(nativeStr)

	str := C.GoString(nativeStr)
	if strings.HasPrefix(str, "parser error") {
		return nil, &parserError{msg: str}
	}
	if strings.HasPrefix(str, "error") {
		return nil, errors.New(str)
	}

	var data ObjectTreeType
	if err := json.Unmarshal([]byte(str), &data); err != nil {
		return nil, fmt.Errorf("unable to deserialize environment: %w", err)
	}

	return &data, nil
}

type IconMetadata struct {
	Width, Height int
	States        []*IconState
}

type IconState struct {
	Name         string
	Dirs, Frames int
}

func ParseIconMetadata(iconPath string) (*IconMetadata, error) {
	nativePath := C.CString(iconPath)
	defer C.free(unsafe.Pointer(nativePath))

	nativeStr := C.SdmmParseIconMetadata(nativePath)
	defer C.SdmmFreeStr(nativeStr)

	str := C.GoString(nativeStr)
	if strings.HasPrefix(str, "error") {
		return nil, errors.New(str)
	}

	var data IconMetadata
	if err := json.Unmarshal([]byte(str), &data); err != nil {
		return nil, fmt.Errorf("unable to deserialize icon metadata: %w", err)
	}

	return &data, nil
}
