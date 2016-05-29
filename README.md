# autobuffer

autobuffer is a small utility you can use to automatically buffer and stream video files over HTTP. It streams to a local, on-disk file.  It is only concerned with streaming the data and makes no assumptions about video format.  As such, you must provide autobuffer with the `-duration` flag to receive accurate feedback on how long you should wait to play the streamed file.  Durations are parsed using golang's `time`, so values like `30m`, `1h50m`, etc, all work as expected.  HTTP basic auth is also supported.

## Example Usage

```
./autobuffer -duration 1h47m -out hackers.mkv -url http://localhost:8080/hackers.mkv
```

`autobuffer` will measure your available downstream bandwidth with the remote url, calculate how long it will take to buffer sufficiently to play without interrupt, and start streaming the target file from the server to your local disk.  To play the video, simply use any video player (I tested mplayer and VLC) to open the file at the `-out` path you specified.  Note that you must use the correct extension in `-out` or some players may have trouble playing the file.  For all the available flags, just run `autobuffer` with no arguments.

## Inspiration

This is a weekend (6 hours on a sunday) hack I put together because of how absolutely terrible VLC is at streaming high-bitrate video over HTTP.

## License

The MIT License (MIT)
