package log

import (
	"fmt"
	"runtime"
)

// enumeration of supported colors
const (
	TextBlack = iota + 30
	TextRed
	TextGreen
	TextYellow
	TextBlue
	TextMagenta
	TextCyan
	TextWhite
)

const colorFormatter = "\x1b[0;%dm%s\x1b[0m"

// return processed text with specified color.
func coloredText(color int, str string) string {
	if isWindows() {
		return str
	}
	if color >= TextBlack && color <= TextWhite {
		return fmt.Sprintf(colorFormatter, color, str)
	}
	return str
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
