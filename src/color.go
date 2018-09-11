package src

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

const colorFormatTemplate = "\x1b[0;%dm%s\x1b[0m"

// return processed text with specified color.
func coloredText(level int, str string) string {
	if isWindows() {
		return str
	}
	switch level {
	case LevelInfo:
		return fmt.Sprintf(colorFormatTemplate, TextWhite, str)
	case LevelDebug:
		return fmt.Sprintf(colorFormatTemplate, TextCyan, str)
	case LevelWarning:
		return fmt.Sprintf(colorFormatTemplate, TextYellow, str)
	case LevelError:
		return fmt.Sprintf(colorFormatTemplate, TextMagenta, str)
	case LevelFatal, LevelPanic:
		return fmt.Sprintf(colorFormatTemplate, TextRed, str)
	default:
		return str
	}
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
