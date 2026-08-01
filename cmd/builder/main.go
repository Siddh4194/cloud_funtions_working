package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/moby/moby/client"
)

func main() {
	ctx := context.Background()
	apiClient, err := client.New(client.FromEnv,client.WithUserAgent("myagent/0.0.0.1"))
	if err != nil {
		log.Fatal(err)
	}
	defer apiClient.Close()

	buildFile := "./app"

	tarStream, err := createTarStream(buildFile)
	if err != nil {
		log.Fatal(err)
	}


	imageOptions := client.ImageBuildOptions{
		Dockerfile: "dockerfile",
		Tags: []string{"my-image:latest"},
		NoCache: false,
		Remove: true,
	}

	imageBuildResponse, err := apiClient.ImageBuild(ctx,tarStream,imageOptions)

	if err != nil {
		log.Fatal(err)
	}

	defer imageBuildResponse.Body.Close()

	io.Copy(os.Stdout,imageBuildResponse.Body)
	storeTheImageToCloudStorage(ctx,"my-image")
}


func storeTheImageToCloudStorage(ctx context.Context,imageName string) error {
	apiCLient, err := client.New(client.FromEnv,client.WithUserAgent("myagent/0.0.01"))
	if err != nil {
		return err
	}

	imageStream , err := apiCLient.ImageSave(ctx,[]string{imageName})
	if err != nil {
		return err
	}

	fileLocation := fmt.Sprintf("bucket/%s.tar",imageName)
	file,err := os.Create(fileLocation)
	if err != nil {
		return err
	}

	_,err = io.Copy(file,imageStream)
	if err != nil {
		return err
	}

	_, err = apiCLient.ImageRemove(ctx,imageName,client.ImageRemoveOptions{Force:true,PruneChildren:true})

	// file.Close()
	// streamFile(ctx, fileLocation)

	// os.Remove(fileLocation)
	return nil
}

func streamFile(ctx context.Context,filePath string) error {
	serverUrl := "http://localhost:4001/upload"
	file, err := os.Open(filePath)
	if err !=  nil {
		return err
	}

	defer file.Close()

	req,err := http.NewRequestWithContext(ctx,"POST",serverUrl,file)

	if err != nil {
		return err
	}

	fileName := filepath.Base(file.Name())

	req.Header.Set("X-File-Name",fileName)	
	req.ContentLength = -1

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK {
		fmt.Println("\nUploaded successfully!")
	} else {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned error %d: %s", resp.StatusCode, string(body))
	}

	defer resp.Body.Close()

	return nil
}

func createTarStream(sourceDir string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	err := filepath.Walk(sourceDir, func(filePath string, finfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, filePath)

		if err != nil {
			return err
		}
		
		header, err := tar.FileInfoHeader(finfo,finfo.Name())

		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if finfo.Mode().IsDir(){
			return nil
		}
		file ,err := os.Open(filePath)
		if err != nil {
			return err
		}
		
		defer file.Close()

		if _, err := io.Copy(tw,file); err != nil {
			return err
		}
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("error creating tar stream: %v", err)
	}
	return buf, nil
}