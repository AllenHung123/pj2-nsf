# Network Service Function

This is a network function used for network service register/de-register/discovery.

## SBI Api Endpoint Point

### API Table

| Service | Endpoint | Description | Model |
| - | - | - | - |
| Heartbeat | `/sbi/heartbeat` | health check | - |
| NF Management | `/sbi/nnsf-nfm` | register & de-register network service | [nnsf-nfm](#nnsf-nfm) |
| Service Discovery | `/sbi/nnsf-disc` | discover the service information | [nnsf-disc](#nnsf-disc) |
| Group Service Discovery | `/sbi/nnsf-disc/group` | discover multiple services information | [nnsf-disc](#nnsf-disc) |

### API Structure

```bash
/sbi
    └─/heartbeat(GET)
    └─/nnsf-nfm(POST\DELETE)
    └─/nnsf-disc(Get)
        └─/group(Get)
```

## Make Command

| Command | Description |
| - | - |
| `make` | default set to `make bin` |
| `make bin` | build the executable binary file and output under **build** folder |
| `make clean` | clean up the build folder |
| `make lint` | run golang-lint |
| `make test` | run go test through the whole repo |
| `make tidy` | run module tidy |
| `make run` | run the NSF binary file under **build** folder with [config.yaml](./config.yaml) |
| `make cleandb` | remove the bbolt db file |
| `make cleantestdb` | remove the test bbolt db files |

## What you need TODO?

There are `TODO` keywords at sections that you need to implement. So, you can try to search the `TODO` keywords.

1. `internal/nsfContext/serviceContext.go`: This file is the core context structure for manage the NF services. You have to implement it correctly which can pass the unit test in `serviceContext_test.go` and the integration test with the docker image provided by CNDI.
2. `internal/db/customDb.go`: You can implement your customized `dbIf` at this file and use this DB to test your service is stored correctly.
3. Use the `make` related command to make sure everything is correct.
4. Use the docker image provided by CNDI to validate if your NSF is implement correctly.

## Docker Image Validate

- Pull the image

    ```bash
    docker pull alonza0314/nsf-tester:latest
    ```

- Run the image for testing with local directory

    ```bash
    docker run --rm -v <absolute path to your local directory>:/work alonza0314/nsf-tester:latest
    ```

## SBI Model

### nnsf-nfm

- POST Request

    ```go
    type NnsfNfmPostRequest struct {
        NfName     string   `json:"nf_name"`
        SbiAddress string   `json:"sbi_address"`
        SbiPort    int      `json:"sbi_port"`
        Services   []string `json:"services"`
    }
    ```

- POST Response

    ```go
    type NnsfNfmPostResponse struct {
        ServiceToken string `json:"service_token"`
        ServiceUuid  string `json:"service_uuid"`
    }
    ```

- DELETE Request

    ```go
    type NnsfNfmDeleteRequest struct {
        NfName   string   `json:"nf_name"`
        Services []string `json:"services"`
    }
    ```

### nnsf-disc

- GET Request

    ```go
    type NnsfDiscGetRequest struct {
        Service string `json:"service"`
    }

    type NnsfDiscGroupGetRequest struct {
        Services []string `json:"services"`
    }
    ```

- GET Response

    ```go
    type NnsfDiscGetResponse struct {
        Service    string `json:"service"`
        NfName     string `json:"nf_name"`
        SbiAddress string `json:"sbi_address"`
        SbiPort    int    `json:"sbi_port"`
    }

    type NnsfDiscGroupGetResponse struct {
        Services []NnsfDiscGetResponse `json:"services"`
    }
    ```

## PJ2 implementation and local review

The completed context stores durable per-NF JSON records and supports partial
service deletion, restart recovery, and concurrent registration. `config.yaml`
continues to use bbolt. See [Report.md](Report.md) for the implementation,
verification evidence, and the current local-only review status.

MongoDB is an optional `DbIf` backend selected with `db_type: "mongodb"`.
Connection settings are `NSF_MONGO_URI` (default `mongodb://127.0.0.1:27017`)
and `NSF_MONGO_DATABASE` (default `pj2_nsf`). The demo config uses port 18080.

```sh
docker run -d --rm --name pj2-nsf-mongo -p 127.0.0.1:27019:27017 mongo:8.0
NSF_TEST_MONGODB=1 NSF_MONGO_URI=mongodb://127.0.0.1:27019 go test -v ./internal/db
make
python3 scripts/check_api.py --backend mongodb
docker stop pj2-nsf-mongo
```

MongoDB tests use a separate `pj2_nsf_test_<pid>` database. The HTTP demo uses a
separate `pj2_nsf_demo_<pid>` database unless explicitly overridden. The optional
`scripts/record_terminal.py` captures live PTY output into MP4; it needs Pillow,
FFmpeg (`FFMPEG` may specify its path), and the DejaVu Sans Mono font. Example:

```sh
NSF_DEMO_PAUSE=0.6 python3 scripts/record_terminal.py --out artifacts/db.mp4 \
  python3 scripts/check_api.py --backend mongodb
```
