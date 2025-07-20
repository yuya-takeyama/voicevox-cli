# voicevox-cli

CLI for [VOICEVOX](https://voicevox.hiroshiba.jp).

## install

```shell
> go install github.com/yuya-takeyama/voicevox-cli@latest
```

## usage

prerequired:

```shell
docker run -d -p 50021:50021 hiroshiba/voicevox_engine:cpu-ubuntu20.04-0.10.4
```

example:

```shell
> voicevox-cli -speaker=0 -style=0 "こんにちは"
main.go:170: 四国めたん ノーマル 2
```

## Features

### Audio Caching

Generated audio files are automatically cached to avoid redundant synthesis. The cache key is generated from:
- Text content
- Speaker ID
- All voice parameters (speed, pitch, intonation, volume)

Cached files are stored in the `audio` directory by default.

### Environment Variables

- `VOICEVOX_CLI_AUDIO_DIR`: Set custom audio cache directory (default: `./audio`)
  - The directory must exist before running the program
  - Example: `VOICEVOX_CLI_AUDIO_DIR=/tmp/voicevox-cache voicevox-cli "こんにちは"`

### Options

- `-endpoint`: API endpoint (default: `http://localhost:50021`)
- `-speaker`: Speaker index (default: `0`)
- `-style`: Style index (default: `0`)
- `-speed`: Speed scale (default: `1.0`)
- `-pitch`: Pitch scale (default: `0.0`)
- `-intonation`: Intonation scale (default: `1.0`)
- `-volume`: Volume scale (default: `1.0`)
- `-o`: Output to WAV file instead of playing audio
  - Example: `voicevox-cli -o output.wav "こんにちは"`
