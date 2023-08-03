# go-aws-s3

This is a small project showing how to interact with AWS S3 with Go. It's not intended to be a fully working application, rather one that shows how various aspects work, which underpins an upcoming [Twilio tutorial](https://www.twilio.com/blog?tag=go).

## Requirements

- An [AWS account](https://docs.aws.amazon.com/AmazonS3/latest/userguide/setting-up-s3.html)
- An [AWS S3 bucket](https://docs.aws.amazon.com/AmazonS3/latest/userguide/create-bucket-overview.html)
- Prior experience with AWS and S3 would be ideal

## Usage

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

Then, upload a file from your local filesystem to your S3 bucket using your preferred tool of choice. 
Below, you can see an example which uses [curl](https://curl.se/). 

```bash
curl -X POST -F file=@<<path/to/your/file>> http://localhost:3000/
```