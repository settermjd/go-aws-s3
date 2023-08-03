# go-aws-s3

This is a small project showing how to interact with AWS S3 with Go. It's not intended to be a fully working application, rather one that shows how various aspects work, which underpins an upcoming [Twilio tutorial](https://www.twilio.com/blog?tag=go).

## Requirements

- An [AWS account](https://docs.aws.amazon.com/AmazonS3/latest/userguide/setting-up-s3.html)
- An [AWS S3 bucket](https://docs.aws.amazon.com/AmazonS3/latest/userguide/create-bucket-overview.html)
- Prior experience with AWS and S3 would be ideal

## Setup/Configuration

After cloning the repository, create a new file in the top-level directory named _.env_ and add the following code to it:

```ini
S3_BUCKET="<<S3_BUCKET>>"
S3_REGION="<<S3_REGION>>"
```

Then, replace the two placeholders with the name of your S3 bucket and its [region](https://github.com/aws/aws-cli/issues/3864#issuecomment-454312681), respectively.
When that's done, start the application running by running the following command.

```bash
go run main.go
```

## Usage

### Upload an image

Then, upload a file from your local filesystem to your S3 bucket using your preferred tool of choice. 
Below, you can see an example which uses [curl](https://curl.se/). 

```bash
curl -X POST -F file=@<<path/to/your/file>> http://localhost:3000/
```

### List all images

To list all images, make a GET request to the default endpoint `/`, as in the example below.

```bash
curl http://localhost:3000/
```

You should see output similar to the following, on success, listing the name, size, and last modified time of each item in the bucket:

```bash
{
    "error": false,
    "msg": [
        {
            "Key": "file_1.jpg",
            "Size": 277150,
            "LastModified": "2023-05-23T14:01:03Z"
        },
        {
            "Key": "file_2.JPG",
            "Size": 1624105,
            "LastModified": "2023-08-03T07:25:42Z"
        },
        {
            "Key": "file_3.txt",
            "Size": 6,
            "LastModified": "2023-06-20T09:16:01Z"
        }
    ]
}
```