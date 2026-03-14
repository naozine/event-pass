//go:build ignore

package main

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	f.SetCellValue(sheet, "A1", "名前")
	f.SetCellValue(sheet, "B1", "メールアドレス")
	f.SetCellValue(sheet, "C1", "ロール")

	for i := 1; i <= 100; i++ {
		row := i + 1
		name := fmt.Sprintf("LoadTest User %d", i)
		email := fmt.Sprintf("loadtest-%04d@loadtest.example.com", i)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), name)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), email)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), "viewer")
	}

	if err := f.SaveAs("loadtest/loadtest_users.xlsx"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("loadtest/loadtest_users.xlsx を作成しました（100件）")
}
