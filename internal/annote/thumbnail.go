package annote

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// Code to create thumbnail images for files.

//

type Thumbnail struct {
	Source    *Store
	item      string
	imagefile string
	thumbdir  string
}

var (
	ErrNoContent = errors.New("No content")
)

// Scale down srcImage to fit the 800x600px bounding box.
//dstImageFit := imaging.Fit(srcImage, 800, 600, imaging.Lanczos)

// DoPDF will read the given content file from the store, create a
// thumbnail, and then copy it back into the store under the key
// "$item-thumbnail"
func (t *Thumbnail) DoPDF(item string) error {
	t.item = item
	itempath := t.Source.Find(item)
	if itempath == "" {
		return ErrNoContent
	}

	tmpfile, err := t.pdf2png(itempath)
	if tmpfile != "" {
		defer os.Remove(tmpfile)
	}
	if err != nil {
		return err
	}

	return t.image2thumb(tmpfile)
}

func (t *Thumbnail) DoImage(item string) error {
	t.item = item

	itempath := t.Source.Find(item)
	if itempath == "" {
		return ErrNoContent
	}

	return t.image2thumb(itempath)
}

func (t *Thumbnail) pdf2png(sourceFilename string) (string, error) {
	tmpfile, err := ioutil.TempFile("", "annote-pdf.*.png")
	if err != nil {
		return "", err
	}
	tmpfile.Close()

	cmd := exec.Command("gs",
		"-sDEVICE=png16m",
		"-sPageList=1",
		"-dUseCropBox",
		"-r300",
		"-o", tmpfile.Name(),
		sourceFilename,
	)

	// run and wait for command to finish
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("PDF to PNG", err)
		log.Println(string(output))
		os.Remove(tmpfile.Name())
		return "", err
	}

	return tmpfile.Name(), nil
}

// image2thumb creates a thumbnail for the given file and then saves it into the store t has under the key "$item-thumbnail".
func (t *Thumbnail) image2thumb(sourceFilename string) error {
	tmpdir, err := ioutil.TempDir("", "annote-thumb")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	// this command is lifted from
	// https://www.smashingmagazine.com/2015/06/efficient-image-resizing-with-imagemagick/
	cmd := exec.Command("mogrify",
		"-path", tmpdir,
		"-filter", "Triangle",
		"-define", "filter:support=2",
		"-thumbnail", "338", // width curate uses
		"-unsharp", "0.25x0.08+8.3+0.045",
		"-dither", "None",
		"-posterize", "136",
		"-quality", "82",
		"-define", "jpeg:fancy-upsampling=off",
		"-define", "png:compression-filter=5",
		"-define", "png:compression-level=9",
		"-define", "png:compression-strategy=1",
		"-define", "png:exclude-chunk=all",
		"-interlace", "none",
		"-colorspace", "sRGB",
		sourceFilename,
	)

	// run and wait for command to finish
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("PNG to Thumbnail", err)
		log.Println(string(output))
		return err
	}

	// now copy the file into the store
	w, err := t.Source.Create(t.item + "-thumbnail")
	if err != nil {
		return err
	}
	defer w.Close()
	r, err := os.Open(filepath.Join(tmpdir, filepath.Base(sourceFilename)))
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(w, r)

	return err

}
