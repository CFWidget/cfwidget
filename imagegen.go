package main

import (
	"bytes"
	"context"
	_ "embed"
	"github.com/cfwidget/cfwidget/curseforge"
	"github.com/cfwidget/cfwidget/widget"
	"github.com/golang/freetype/truetype"
	"go.elastic.co/apm/v2"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/png"
	"math"
)

const ThumbnailSize = 256

var (
	//dpi              = flag.Float64("dpi", 72, "screen resolution in Dots Per Inch")
	//size             = flag.Float64("size", 16, "font size in points")
	//spacing          = flag.Float64("spacing", 1.5, "line spacing (e.g. 2 means double spaced)")
	dpi              float64 = 144
	size             float64 = 16
	spacing          float64 = 1.5
	thumbnailPadding int     = 8

	//go:embed FreeSans.ttf
	regularFontData []byte
	standardFont    = getFont(regularFontData)

	//go:embed FreeSansBold.ttf
	boldFontData []byte
	boldFont     = getFont(boldFontData)
)

type ImageRequest struct {
	DarkMode    bool
	Transparent bool
	NoThumbnail bool
}

func generateImage(project *widget.ProjectProperties, request ImageRequest, ctx context.Context) ([]byte, error) {
	thumbnailSize := ThumbnailSize

	var thumbnail image.Image
	var err error
	if request.NoThumbnail {
		thumbnailSize = 0
	} else {
		thumbnail, err = curseforge.GetThumbnail(project.Thumbnail, ctx)
		if err != nil {
			return nil, err
		}
	}

	span, _ := apm.StartSpan(ctx, "generateImage", "custom")
	defer span.End()

	gameName := project.Game
	game := curseforge.GetGameBySlug(gameName)
	if game.Name != "" {
		gameName = game.Name
	}

	text := []Text{
		{
			Font: boldFont,
			Text: project.Title,
		},
		{
			Font:    standardFont,
			Text:    " by " + project.Members[0].Username,
			EndLine: true,
		},
		{
			Font: boldFont,
			Text: "Latest File:",
		},
		{
			Font:    standardFont,
			Text:    " " + project.Download.Name,
			EndLine: true,
		},
		{
			Font: boldFont,
			Text: "For:",
		},
		{
			Font:    standardFont,
			Text:    " " + gameName + " " + project.Download.Version,
			EndLine: true,
		},
		{
			Font: boldFont,
			Text: "Downloads:",
		},
		{
			Font:    standardFont,
			Text:    " " + messagePrinter.Sprintf("%d", project.Downloads["total"]),
			EndLine: true,
		},
		{
			Font: boldFont,
			Text: "Uploaded:",
		},
		{
			Font:    standardFont,
			Text:    " " + project.Download.UploadedAt.Format("January 02 2006, 03:04pm") + " UTC",
			EndLine: true,
		},
	}

	output := new(bytes.Buffer)

	//prepare white box as final result
	imageXSize := 928 + thumbnailPadding
	imageYSize := ThumbnailSize //use original size for our heights
	if !request.NoThumbnail {
		imageXSize = imageXSize + thumbnailSize + thumbnailPadding
		imageYSize = imageYSize + (2 * thumbnailPadding)
	}

	finalImage := image.NewRGBA(image.Rect(0, 0, imageXSize, imageYSize))

	bgColor := image.White
	if request.DarkMode {
		bgColor = image.Black
	}

	if !request.Transparent {
		draw.Draw(finalImage, finalImage.Bounds(), bgColor, image.Point{X: 0, Y: 0}, draw.Src)
		draw.Draw(finalImage, image.Rect(0, 0, (thumbnailPadding*2)+thumbnailSize, (thumbnailPadding*2)+thumbnailSize), bgColor, image.Point{X: 0, Y: 0}, draw.Src)
	}

	//add thumbnail image
	if !request.NoThumbnail {
		scaled := image.NewRGBA(image.Rect(0, 0, thumbnailSize, thumbnailSize))
		draw.BiLinear.Scale(scaled, scaled.Rect, thumbnail, thumbnail.Bounds(), draw.Over, nil)
		draw.Draw(finalImage, image.Rect(thumbnailPadding, thumbnailPadding, thumbnailPadding+thumbnailSize, thumbnailPadding+thumbnailSize), scaled, image.Point{X: 0, Y: 0}, draw.Src)
	}

	d := &font.Drawer{
		Dst: finalImage,
		Src: image.Black,
	}

	if request.DarkMode {
		d.Src = image.White
	}

	textOffset := thumbnailSize + (thumbnailPadding * 2)
	if request.NoThumbnail {
		textOffset = thumbnailPadding
	}
	y := 10 + int(math.Ceil(size*dpi/72))
	dy := int(math.Ceil(size * spacing * dpi / 72))
	d.Dot = fixed.P(textOffset, y)

	for _, s := range text {
		d.Face = s.Font
		d.DrawString(s.Text)
		if s.EndLine {
			y += dy
			d.Dot = fixed.P(textOffset, y)
		}
	}

	err = png.Encode(output, finalImage)
	return output.Bytes(), err
}

func getFont(fontData []byte) font.Face {
	parsedFont, err := truetype.Parse(fontData)
	if err != nil {
		panic(err)
	}

	return truetype.NewFace(parsedFont, &truetype.Options{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingNone,
	})
}

type Text struct {
	Font    font.Face
	Text    string
	EndLine bool
}

func ParseHexColorFast(s string) (c color.RGBA) {
	c.A = 0xff

	if s[0] != '#' {
		return
	}

	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		return 0
	}

	switch len(s) {
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
	}
	return
}
