# muSync

Lightweight music playlist synchnonization container.

## Usage

### Configuration

The app expects a `config.yml` file, in which you can configure your playlists and other options. See the [example-config.yml](./example-config.yml) for available options.

The app created a `state.json` file, in which it tracks internal playlist state, downloaded songs, filenames... You do not need to touch this file during normal operation.

Deleting the `state.json` file will cause the app to forget all downloaded files and start from scratch.

Both the `config.yml` and the `state.json` files are stored in the `musync` folder, next to the executable.

### Docker

This app is designed to work as a Docker container. See the provided `docker-compose.yml` for an example setup.

The `/app/musync` volume mount should contain your `config.yml` file. The `state.json` file will also be created there.

### Other

While running this app outside a Docker container is not supported, nothing should prevent you from doing it. Read the docker setup instructions, the Dockerfile, and build the app yourself.
