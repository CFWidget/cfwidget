package main

import (
	"flag"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"image"
	"image/png"
	"math"
	"os"
)

const ThumbnailSize = 128

var (
	dpi     = flag.Float64("dpi", 72, "screen resolution in Dots Per Inch")
	size    = flag.Float64("size", 16, "font size in points")
	spacing = flag.Float64("spacing", 1.5, "line spacing (e.g. 2 means double spaced)")
)

func main() {
	flag.Parse()

	thumbnail := "635421614078544069.png"
	name := "Journeymap"
	author := "techbrew"
	gameVersion := "1.19.3"
	fileName := "journeymap-1.19.3-5.9.1-forge.jar"
	downloads := "158,917,925"
	game := "Minecraft"

	standardFont := getFont("FreeSans.ttf")
	boldFont := getFont("FreeSansBold.ttf")

	text := []Text{
		{
			Font: boldFont,
			Text: name,
		},
		{
			Font:    standardFont,
			Text:    " by " + author,
			EndLine: true,
		},
		{
			Font: boldFont,
			Text: "Latest File:",
		},
		{
			Font:    standardFont,
			Text:    " " + fileName,
			EndLine: true,
		},
		{
			Font: boldFont,
			Text: "For:",
		},
		{
			Font:    standardFont,
			Text:    " " + game + " " + gameVersion,
			EndLine: true,
		},
		{
			Font: boldFont,
			Text: "Downloads:",
		},
		{
			Font:    standardFont,
			Text:    " " + downloads,
			EndLine: true,
		},
	}

	imgFile1, err := os.Open(thumbnail)
	if err != nil {
		panic(err)
	}
	defer imgFile1.Close()

	output, _ := os.Create("result.png")
	if err != nil {
		panic(err)
	}
	defer output.Close()

	image1, _, _ := image.Decode(imgFile1)

	//scale thumbnail to a constant size
	scaled := image.NewRGBA(image.Rect(0, 0, ThumbnailSize, ThumbnailSize))
	draw.BiLinear.Scale(scaled, scaled.Rect, image1, image1.Bounds(), draw.Over, nil)

	//prepare white box as final result
	finalImage := image.NewRGBA(image.Rect(0, 0, 600, 144))
	draw.Draw(finalImage, finalImage.Bounds(), image.White, image.Point{X: 0, Y: 0}, draw.Src)

	//add thumbnail image
	draw.Draw(finalImage, image.Rect(8, 8, 8+ThumbnailSize, 8+ThumbnailSize), scaled, image.Point{X: 0, Y: 0}, draw.Src)

	d := &font.Drawer{
		Dst: finalImage,
		Src: image.Black,
	}

	textOffset := ThumbnailSize + 8 + 8
	y := 10 + int(math.Ceil(*size**dpi/72))
	dy := int(math.Ceil(*size * *spacing * *dpi / 72))
	d.Dot = fixed.P(textOffset, y)

	for _, s := range text {
		d.Face = s.Font
		d.DrawString(s.Text)
		if s.EndLine {
			y += dy
			d.Dot = fixed.P(textOffset, y)
		}
	}

	_ = png.Encode(output, finalImage)
}

func getFont(filename string) font.Face {
	fontBytes, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	fontData, err := truetype.Parse(fontBytes)
	if err != nil {
		panic(err)
	}

	return truetype.NewFace(fontData, &truetype.Options{
		Size:    *size,
		DPI:     *dpi,
		Hinting: font.HintingNone,
	})
}

type Text struct {
	Font    font.Face
	Text    string
	EndLine bool
}
