# captured

This library allows to capture screenshot of a specific window.

## How to Use

```go
package main

import (
	"fmt"
	"image/jpeg"
	"os"
)

func main() {
	img, err := Capture.CaptureWindowByTitle("The Window Title", FullWindow)
	if err != nil {
		panic(err)
	}

	file, err := os.Create("test.jpg")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 95}); err != nil {
		panic(err)
	}
}
```
