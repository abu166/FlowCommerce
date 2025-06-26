how to read exchange file:

```bash
docker load -i exchange/exchange1_amd64.tar
```

after this you will see:

```bash
08000c18d16d: Loading layer [==================================================>]  3.642MB/3.642MB
5f70bf18a086: Loading layer [==================================================>]      32B/32B
23feda13f397: Loading layer [==================================================>]   2.09MB/2.09MB
```

check the images:
```bash
docker images
```

inspect the exchange:
```bash
docker inspect exchange1:latest
```

to run the exchange server:
```bash
docker run -d -p 8080:40101 --name exchange-server exchange1:latest
```
to check if exchange-server is running:

```bash
docker logs exchange-server
```

see localhost:8080 to see the exchange 

to stop the server exchange running:

```bash
docker stop my-exchange exchange-server
docker rm my-exchange exchange-server
```