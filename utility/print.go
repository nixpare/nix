package utility

import "fmt"

func PrintBytes(data ...int) string {
	var size float64
	for _, n := range data {
		size += float64(n)
	}

	units := [...]string{ "B", "KB", "MB", "GB" }
	var unitIndex int
	for size >= 1024. && unitIndex < 4 {
		size /= 1024
		unitIndex ++
	}

	return fmt.Sprintf("%3.3f %s", size, units[unitIndex])
}
