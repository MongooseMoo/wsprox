# wsprox for MUD/MOO

This package, wsprox, provides a simple and efficient way to proxy websocket connections to a regular TCP server, such as a MUD (Multi-User Dungeon) or MOO (MUD, Object-Oriented). It is designed to be easy to set up and use, allowing for seamless communication between web-based clients and traditional MUD/MOO servers.

## Features

- Easy to configure with environment variables or command-line flags.
- Supports both IPv4 and IPv6 connections.
- Efficient forwarding of messages with adjustable buffer sizes.
- Handles the initial PROXY protocol line to transmit client information to the server.

## Usage

To start the proxy, simply run the compiled binary with the appropriate flags or environment variables set to configure the upstream server address, upstream server port, listen address, and buffer size.

Example:

```sh
./wsprox -upstream mud.example.com -upstream-port 4000 -listen-address 0.0.0.0:8080 -buffer-size 8192
```

Alternatively, you can use environment variables:

```sh
export WSPROX_UPSTREAM=mud.example.com
export WSPROX_UPSTREAM_PORT=4000
export WSPROX_LISTEN_ADDRESS=0.0.0.0:8080
export WSPROX_BUFFER_SIZE=8192
./wsprox
```

## Building from Source

To build the project from source, ensure you have Go installed and run:

```sh
go build -o wsprox
```

This will compile the source code into a binary named `wsprox`.

## Contributing

Contributions to this project are welcome. Please feel free to open issues or submit pull requests.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
