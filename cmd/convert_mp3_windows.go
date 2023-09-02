//go:build windows

package cmd

import (
	"fmt"

	"github.com/braheezy/goqoa/pkg/qoa"
)

func decodeMp3(inputData *[]byte) ([]int16, *qoa.QOA) {
	fmt.Println("MP3 is not supported on Windows")
	return nil, nil
}

func encodeMp3(outputFile string, q *qoa.QOA, decodedData []int16) {
	fmt.Println("MP3 is not supported on Windows")
}
