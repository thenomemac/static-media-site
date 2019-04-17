package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const htmlTemplate = `<!DOCTYPE html>
<html>
  <head>
    <title>UnPlex</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/4.7.0/css/font-awesome.min.css">
    <style>
     * {
       box-sizing: border-box;
     }
     
     #myInput {
       background-position: 10px 10px;
       background-repeat: no-repeat;
       width: 100%;
       font-size: 16px;
       padding: 12px 20px 12px 40px;
       border: 1px solid #ddd;
       margin-bottom: 12px;
     }
     
     #myTable {
       border-collapse: collapse;
       width: 100%;
       border: 1px solid #ddd;
       font-size: 18px;
     }
     
     #myTable th, #myTable td {
       text-align: left;
       padding: 12px;
     }
     
     #myTable tr {
       border-bottom: 1px solid #ddd;
     }
     
     #myTable tr.header, #myTable tr:nth-child(even) {
       background-color: #f1f1f1;
     }

     img {
       max-width: 100%;
       height: auto;
     }

     .showme {
       display: none;
     }

     .showhim:hover .showme {
       display: block;
     }

     a {
       text-decoration: none; 
       color: black; 
     }
    </style>
  </head>
  <body>
    <h2><a href="/">UnPlex</a></h2>
    <input type="text" id="myInput" onkeyup="myFunction()" placeholder="Search videos..">

    <table id="myTable">
      <tr class="header">
        <th style="width:95%;">Videos</th>
        <th style="width:5%;"></th>
      </tr>
      {{range .DirNames}}
      <tr>
        <td>
          <a href="{{.}}"><p><b>{{.}}</b></p></a>
        </td>
        <td>
        </td>
      </tr>
    {{end}}
    {{range .FileNames}}
      <tr>
        <td>
          <div class="showhim">
            <div class="showme">
              <img src="thumbs/{{.}}.jpg">
            </div>
            <p>{{.}}</p>
            <td>
              <a href="{{.}}"><i class="fa fa-play" style="font-size:24px"></i></a>
            </td>
          </div>
        </td>
      </tr>
    {{end}}
    </table>
    <script>
     function myFunction() {
       // Declare variables
       var input, filter, table, tr, td, i, txtValue;
       input = document.getElementById("myInput");
       filter = input.value.toUpperCase();
       table = document.getElementById("myTable");
       tr = table.getElementsByTagName("tr");
       
       // Loop through all table rows, and hide those who don't match the search query
       for (i = 0; i < tr.length; i++) {
         td = tr[i].getElementsByTagName("td")[0];
         if (td) {
           txtValue = td.textContent || td.innerText;
           if (txtValue.toUpperCase().indexOf(filter) > -1) {
             tr[i].style.display = "";
           } else {
             tr[i].style.display = "none";
           }
         }
       }
     }
    </script>
  </body>
</html>
`

const moviePreviewScript = `#!/bin/bash -ex
(     
    if [ -z "$1" ]; then
        echo "usage: ./movie_preview.sh VIDEO [HEIGHT=120] [COLS=100] [ROWS=1] [OUTPUT]"
        exit
    fi
     
    MOVIE=$1
    HEIGHT=$2
    COLS=$3
    ROWS=$4
    OUT_FILENAME=$5
     
    # get video name without the path and extension
    MOVIE_NAME=$(basename "$MOVIE")
    OUT_DIR=$(pwd)
     
    if [ -z "$HEIGHT" ]; then
        HEIGHT=120
    fi
    if [ -z "$COLS" ]; then
        COLS=100
    fi
    if [ -z "$ROWS" ]; then
        ROWS=1
    fi
    if [ -z "$OUT_FILENAME" ]; then
        OUT_FILENAME=$(echo ${MOVIE_NAME%.*}_preview.jpg)
    fi
     
    OUT_FILEPATH=$(echo $OUT_DIR/$OUT_FILENAME)
     
    TOTAL_IMAGES=$(echo "$COLS*$ROWS" | bc)
     
    # get total number of frames in the video
    # ffprobe is fast but not 100% reliable. It might not detect number of frames correctly!
    NB_FRAMES=$(ffprobe -show_streams "$MOVIE" 2> /dev/null | grep nb_frames | head -n1 | sed 's/.*=//')
     
    if [ "$NB_FRAMES" = "N/A" ]; then
        # as a fallback we'll use ffmpeg. This command basically copies this
        # video to /dev/null and it counts frames in the process.
        # It's slower (few seconds usually) than ffprobe but works everytime.
        NB_FRAMES=$(ffmpeg -nostats -i "$MOVIE" -vcodec copy -f rawvideo -y /dev/null 2>&1 | grep frame | awk '{split($0,a,"fps")}END{print a[1]}' | sed 's/.*= *//')
    fi
     
    # calculate offset between two screenshots, drop the floating point part
    NTH_FRAME=$(echo "$NB_FRAMES/$TOTAL_IMAGES" | bc)
    echo "capture every ${NTH_FRAME}th frame out of $NB_FRAMES frames"
     
    # make sure output dir exists
    mkdir -p $OUT_DIR
     
    FFMPEG_CMD="ffmpeg -loglevel panic -i \"$MOVIE\" -y -frames 1 -q:v 1 -vf \"select=not(mod(n\,$NTH_FRAME)),scale=-1:${HEIGHT},tile=${COLS}x${ROWS}\" \"$OUT_FILEPATH\""
     
    eval $FFMPEG_CMD
    echo $OUT_FILEPATH
)
`

const outName = "index.html"
const metaName = "thumbs"

var wdirGlobal = "."

func main() {

	wdir := flag.String("dir", ".", "working directory")

	flag.Parse()

	// check if the source dir exist
	src, err := os.Stat(*wdir)
	if err != nil {
		fmt.Printf("Source does not exist: \"%s\"\n", *wdir)
		os.Exit(1)
	}

	// check if the source is indeed a directory or not
	if !src.IsDir() {
		fmt.Printf("Source is not a directory: \"%s\"\n", *wdir)
		os.Exit(1)
	}

	// TODO: refactor
	wdirGlobal = *wdir
	fmt.Printf("Working directory: \"%s\"\n", *wdir)

	indexDir(*wdir)
}

func indexDir(wdir string) {
	fmt.Println("indexDir called with: ", wdir)
	var filepaths []string
	var dirpaths []string

	fileinfos, err := ioutil.ReadDir(wdir)
	if err != nil {
		panic(err)
	}

	for _, info := range fileinfos {
		path := filepath.Join(wdir, info.Name())
		if filepath.Base(path) != outName && filepath.Base(path) != metaName {
			// assume symlinks are directories
			if info.IsDir() || (info.Mode()&os.ModeSymlink) == os.ModeSymlink {
				indexDir(path)
				dirpaths = append(dirpaths, path)
			} else {
				filepaths = append(filepaths, path)
			}
		}
	}

	dirNames := make([]string, len(dirpaths))
	for i, dir := range dirpaths {
		fmt.Println(dir)
		dirNames[i] = filepath.Base(dir)
	}

	scriptPath := writeTmpScript()

	fileNames := make([]string, len(filepaths))
	for i, file := range filepaths {
		fmt.Println(file)
		err = createThumbnail(file, scriptPath)
		if err != nil {
			panic(err)
		}

		fmt.Println(file)
		fileNames[i] = filepath.Base(file)
	}

	outPath := filepath.Join(wdir, outName)
	f, err := os.Create(outPath)
	if err != nil {
		log.Println("create file err: ", err)
		panic(err)
	}

	tmpl, err := template.New("index").Parse(htmlTemplate)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(f, struct {
		FileNames []string
		DirNames  []string
	}{fileNames, dirNames})
	if err != nil {
		panic(err)
	}
}

func writeTmpScript() string {
	// TODO: implement file cleanup

	// Create our Temp File:  This will create a filename like /tmp/prefix-123456
	// We can use a pattern of "pre-*.txt" to get an extension like: /tmp/pre-123456.txt
	tmpFile, err := ioutil.TempFile(os.TempDir(), "movie_preview_*.sh")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}

	// Remember to clean up the file afterwards
	// defer os.Remove(tmpFile.Name())
	// fmt.Println("Created File: " + tmpFile.Name())

	// Example writing to the file
	text := []byte(moviePreviewScript)
	if _, err = tmpFile.Write(text); err != nil {
		log.Fatal("Failed to write to temporary file", err)
	}

	err = os.Chmod(tmpFile.Name(), 0500)
	if err != nil {
		log.Fatal("Failed to change fiel permissions:", err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}

	return tmpFile.Name()
}

func createThumbnail(path string, scriptPath string) error {
	thumbName := filepath.Base(path) + ".jpg"
	thumbPath := filepath.Join(filepath.Dir(path), metaName, thumbName)

	if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
		err := ensureDir(thumbPath)
		// cmdPath := filepath.Join(wdirGlobal, metaName, scriptPath)
		cmdPath := scriptPath
		cmdArgs := []string{path, "200", "2", "2", thumbPath}
		log.Println(cmdArgs)
		out, err := exec.Command(cmdPath, cmdArgs...).Output()
		if err != nil {
			// TODO: refactor to return
			fmt.Fprintln(os.Stderr, "There was an error: ", err)
			log.Fatalln(err)
		}
		fmt.Println(string(out))
	}

	return nil
}

func ensureDir(path string) error {
	var merr error
	dirName := filepath.Dir(path)
	if _, serr := os.Stat(dirName); serr != nil {
		merr = os.Mkdir(dirName, os.ModePerm)

	}
	return merr
}
