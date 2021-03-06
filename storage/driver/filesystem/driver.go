package filesystem

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	//"time"

	"github.com/alvinfeng/mosaic/storage/cache"
	"github.com/alvinfeng/mosaic/storage/driver"
)

const (
	driverName        = "filesystem"
	defaultRootDir    = "/tmp/buckets"
	defaultBucketSize = 15
)

type Config struct {
	RootDir string `yaml:"rootdir,omitempty"`
}

type driver struct {
	rootDir    string
	bucketSize int
	cache      cache.Cache
}

func New(c Config) (*driver, error) {
	baseDir := defaultRootDir
	if c.RootDir != "" {
		baseDir = c.RootDir
	}

	d := &driver{
		rootDir:    baseDir,
		bucketSize: defaultBucketSize,
		cache:      nil,
	}
	return d, nil
}

func (d *driver) Name() string {
	return driverName
}

func (d *driver) SetCache(c cache.Cache) {
	// TODO: prefill the cache
	d.cache = c
}

func (d *driver) Store(data []byte) error {
	uuid := storagedriver.Encode(data)
	m, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}

	r, g, b := storagedriver.GetAverageColor(m, m.Bounds())
	bucketPath := d.bucketPath(r, g, b)
	fmt.Printf("uuid: %v, format: %v, rgb: (%v, %v, %v), bucket: %v\n", uuid, format, r, g, b, bucketPath)

	err = os.MkdirAll(bucketPath, os.ModePerm)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(bucketPath, fmt.Sprintf("%v.%v", uuid, format)), data, 0644)
	return err
}

func (d *driver) Get(r, g, b uint8) (image.Image, error) {
	var img image.Image
	bucketPath := d.bucketPath(int(r), int(g), int(b))
	if _, err := os.Stat(bucketPath); err != nil {
		if os.IsNotExist(err) {
			// TODO: return some error denoting nothing found
			// for now return a simple flat color
			fmt.Println("Could not find bucket: ", bucketPath)
			drawableImg := image.NewRGBA(image.Rect(0, 0, 150, 150))
			c := color.RGBA{r, g, b, 255}
			draw.Draw(drawableImg, drawableImg.Bounds(), &image.Uniform{c}, image.ZP, draw.Src)
			return drawableImg, nil
		} else {
			return img, err
		}
	}

	// start := time.Now()
	fileNames := []string{}
	cached := false
	if d.cache != nil {
		fileNames, cached = d.cache.Get(bucketPath)
	}

	if !cached {
		files, err := ioutil.ReadDir(bucketPath)
		if err != nil {
			return img, err
		}

		fileNames = []string{}
		for i := 0; i < len(files); i++ {
			item := files[i].Name()
			fileNames = append(fileNames, item)
			if d.cache != nil {
				d.cache.Add(item, bucketPath)
			}
		}
	}
	// elapsed := time.Since(start)
	// fmt.Printf("Bucket (%v, %v, %v) with %v items took %v\n", r, g, b, len(files), elapsed)

	// pick a random image in bucket
	i := rand.Intn(len(fileNames))
	fileName := fileNames[i]

	// return chosen random image
	reader, err := os.Open(filepath.Join(bucketPath, fileName))
	if err != nil {
		return img, err
	}
	defer reader.Close()
	img, _, err = image.Decode(reader)
	return img, err
}

func (d *driver) bucketPath(r, g, b int) string {
	rbucket, gbucket, bbucket := storagedriver.ColorBucket(d.bucketSize, r, g, b)
	return filepath.Join(d.rootDir, fmt.Sprintf("r%v_g%v_b%v", rbucket, gbucket, bbucket))
}
