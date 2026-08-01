package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/moby/moby/client"
)

func DownloadImage(ctx context.Context,imageName string,tags []string) error{
	url := "http://localhost:4001/" + imageName + ".tar"

	response, err := http.Get(url)

	if err != nil {
		return fmt.Errorf("Error while downloading the image %w",err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to download the image, status code: %d", response.StatusCode)
	}

	apiClient, err := client.New(client.FromEnv, client.WithUserAgent("worker"))

	imageLoadOption := client.ImageLoadOption{
    Tags:      tags,
    ImageName: imageName,
	}

	_, err = apiClient.ImageLoad(ctx, response.Body, imageLoadOption)
	if err != nil {
		return fmt.Errorf("Error while loading the image %w", err)
	}

	return nil
}

func StartContainer(imageName string) {
	ctx := context.Background()
	err := DownloadImage(ctx, imageName, []string{imageName})
	if err != nil {
		fmt.Printf("Error while downloading the image: %v\n", err)
	}


}

func main() {
	StartContainer("my-image")
}