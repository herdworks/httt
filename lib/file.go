package hypertemplates

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// HyperFile
//
// A HyperFile is a publishable HyperTemplates resource.
// A HyperFile is a file on disk that may be published via a HyperProvider.
// HyperFiles include rendered HTML files, and their associated assets (media, CSS, JS, etc).
type HyperFile struct {
	URI        *url.URL
	Base       string // source base
	Prefix     string // destination prefix
	Properties map[string]string
}
type HyperFiles []HyperFile

func (f *HyperFile) File() (*os.File) {
	file,err := os.Open(f.URI.Path)
	if err != nil { return nil }
	return file
}

func (f *HyperFile) Bytes() ([]byte, error) {
	file,err := os.ReadFile(f.URI.Path)
	if err != nil { return nil, err }
	return file, nil
}

func (f *HyperFile) Size() (int64) {
	file,err := os.Stat(f.URI.Path)
	if err != nil { return 0 }
	return file.Size()
}

func (f *HyperFile) Extension() (string) {
	return filepath.Ext(f.URI.Path)
}

func (file *HyperFile) AbsBase() (string) {
	absoluteBasePath,_ := filepath.Abs(file.Base)
	return absoluteBasePath + string(filepath.Separator)
}
func (file *HyperFile) AbsDir() (string) {
	absolutePath,_ := filepath.Abs(file.URI.Path)
	return filepath.Dir(absolutePath) + string(filepath.Separator)
}
func (file *HyperFile) AbsPath() (string) {
	absolutePath,_ := filepath.Abs(file.URI.Path)
	return absolutePath
}
func (file *HyperFile) RelDir() (string) {
	pwd,_ := os.Getwd()
	reldir,_ := filepath.Rel(pwd, file.AbsDir())
	return reldir
}
func (file *HyperFile) RelPath() (string) {
	pwd,_ := os.Getwd()
	relpath,_ := filepath.Rel(pwd, file.AbsPath())
	return relpath
}
func (file *HyperFile) SubDir() (string) {
	return filepath.Dir(file.SubPath()) + string(filepath.Separator)
}
func (file *HyperFile) SubPath() (string) {
	return strings.Replace(file.AbsPath(), file.AbsBase(), "", 1)
}
func (file *HyperFile) DestinationDir(prefix string) (string) {
	return filepath.Dir(file.DestinationAbsPath(prefix)) + string(filepath.Separator)
}
func (file *HyperFile) DestinationAbsPath(prefix string) (string) {
	if len(prefix) > 0 { file.Prefix = prefix }
	abspath,_ := filepath.Abs(filepath.Join(file.Prefix, file.SubPath()))
	return abspath
}
func (file *HyperFile) DestinationSubPath(prefix string) (string) {
	if len(prefix) > 0 { file.Prefix = prefix }
	return strings.TrimPrefix(filepath.Join(file.Prefix, file.SubPath()), string(filepath.Separator))
}

func (files HyperFiles) SetRoot(dirPath string) {
	for f := range files { files[f].SetRoot(dirPath) }
}
func (f *HyperFile) SetRoot(dirPath string) {
	f.Base = dirPath
}

func (f *HyperFile) Href(href string) (string) {
	baseURL,_ := url.Parse(href)
	baseURL.Scheme = "https"
	url,_ := url.Parse(f.URI.String()) // make a mutable copy of the HyperFile URL
	url.Path = f.SubPath()
	return baseURL.ResolveReference(url).String()
}

func (f *HyperFile) GetMetadata() map[string]string {
	return f.Properties
}

func (files HyperFiles) SetMetadata(key string, value string) {
	for f := range files { files[f].SetMetadata(key, value) }
}

func (f *HyperFile) SetMetadata(key string, value string) {
	if f.Properties == nil { f.Properties = make(map[string]string) }
	f.Properties[key] = value
}

// Reads first 512 bytes of the file to determine the mimetype;
// NOTE: 512 bytes is the maximum used by http.DetectContentType())
func (f *HyperFile) Mimetype() (mimetype string) {
	mimetype = f.ExtensionType() // use the corresponding MIME type for known file extensions
	if mimetype == "application/octet-stream" { mimetype = f.ContentType() } // fallback to Content-Type metadata
	if mimetype != "application/octet-stream" { return mimetype } // return early if a known or user-provided MIME type is detected
	file := f.File() // open the file
	defer file.Seek(0, io.SeekStart) // reset the file handle after reading
	header := make([]byte, 512)
	_,err := file.Read(header)
	if err == nil { mimetype = http.DetectContentType(header) }
	return mimetype
}

// Return Content-Type metadata as a string
func (f *HyperFile) ContentType() string {
	contenttype := f.GetMetadata()["Content-Type"]
	if contenttype == "" { contenttype = f.GetMetadata()["content-type"] }
	if contenttype == "" { contenttype = "application/octet-stream" }
	return contenttype
}

// Return common MIME types based on file extension
// Reference: https://developer.mozilla.org/en-US/docs/Web/HTTP/MIME_types/Common_types
func (f *HyperFile) ExtensionType() string {
	switch f.Extension() {
		case ".html": return "text/html"
		case ".css": return "text/css"
		case ".js": return "application/javascript"
		case ".json": return "application/json"
		case ".xml": return "application/xml"
		case ".atom": return "application/atom+xml"
		case ".opml": return "text/x-opml"
		case ".svg": return "image/svg+xml"
		case ".ico": return "image/x-icon"
		case ".jpg": return "image/jpeg"
		case ".jpeg": return "image/jpeg"
		case ".gif": return "image/gif"
		case ".png": return "image/png"
		case ".webp": return "image/webp"
		case ".webm": return "video/webm"
		case ".mp4": return "video/mp4"
		case ".mp3": return "audio/mpeg"
		case ".wav": return "audio/wav"
		default: return "application/octet-stream"
	}
}

func (files HyperFiles) Sort(recursive bool) {
	slices.SortFunc(files, func(a, b HyperFile) int {
		if recursive { return cmp.Compare(a.AbsDir(), b.AbsDir()) }
		return cmp.Compare(a.AbsPath(), b.AbsPath())
	})
}

func (files HyperFiles) FileNames() (paths []string) {
	for _, file := range files {
		paths = append(paths, filepath.Base(file.URI.Path))
	}
	return paths
}
func (files HyperFiles) FilePaths() (paths []string) {
	for _, file := range files {
		paths = append(paths, file.SubPath())
	}
	return paths
}
func (files HyperFiles) ColumnValues(field string) (values []string) {
	values = []string{}
	for i := range files {
		switch field {
			case "Source", "source": values = append(values, files[i].RelPath())
			case "Size", "size": values = append(values, fmt.Sprintf("%d", files[i].Size()))
			case "Path", "path": values = append(values, files[i].SubPath())
		}
	}
	return values
}

// =============================================================================
// Helper types, methods, and functions
// =============================================================================

// Walk a directory and return a list of file (sub)paths
func GetPaths(pathname string, recursive bool) (paths []string) {
	dir, err := os.Open(pathname)
	if err != nil { return []string{} }
	defer dir.Close()
	files, err := dir.ReadDir(-1)
	if err != nil { return []string{} }
	for _, f := range files {
		if recursive && f.IsDir() {
			paths = append(paths, GetPaths(filepath.Join(pathname, f.Name()), recursive)...) // RECURSION
		} else {
			if f.IsDir() { continue }
			paths = append(paths, filepath.Join(pathname, f.Name()))
		}
	}
	slices.Sort(paths)
	return paths
}

func GetPathsByPattern(directory string, recursive bool, include string, exclude string) (paths []string) {
	var err error
	var regInclude *regexp.Regexp
	var regExclude *regexp.Regexp
	regInclude,err = regexp.Compile(include)
	if err != nil {
		fmt.Printf("error compiling regular expression from includsive pattern: %s\n\n%s\n", include, err)
		return []string{} // guard against invalid regular expressions
	}
	regExclude,err = regexp.Compile(exclude)
	if err != nil {
		fmt.Printf("error compiling regular expression from exclusive pattern: %s\n\n%s\n", exclude, err)
		return []string{} // guard against invalid regular expressions
	}
	for _, path := range GetPaths(directory, recursive) {
		if len(include) > 0 && !regInclude.MatchString(path) { continue } // skip files that don't match the inclusive pattern
		if len(exclude) > 0 && regExclude.MatchString(path) { continue } // skip files that match the exclusive pattern
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func GetSourcePaths(configfile string) (paths []string) {
	// everything but the build(s)...
	paths = []string{}
	dir := filepath.Dir(configfile)
	for _,p := range GetPaths(dir, true) {
		if strings.HasPrefix(p, ".") { continue }
		if strings.HasPrefix(p, "builds/") { continue }
		paths = append(paths, GetPaths(dir, true)...)
	}
	return paths
}

func GetFiles(directory string, recursive bool) (files HyperFiles) {
	paths := GetPaths(directory, recursive)
	dir,_ := filepath.Abs(directory)
	for _, p := range paths {
		uri,_ := url.Parse(p)
		files = append(files, HyperFile{
			URI: uri,
			Base: dir,
		})
	}
	files.Sort(recursive)
	return files
}

func GetFilesByPattern(directory string, recursive bool, include string, exclude string) (files HyperFiles) {
	paths := GetPathsByPattern(directory, recursive, include, exclude)
	dir,_ := filepath.Abs(directory)
	for _, p := range paths {
		abs,_ := filepath.Abs(p)
		uri,_ := url.Parse(abs)
		files = append(files, HyperFile{ URI: uri, Base: dir })
	}
	files.Sort(recursive)
	return files
}
