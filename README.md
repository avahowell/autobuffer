# autobuffer

autobuffer is a small utility you can use to automatically buffer and stream video files over HTTP.

## But, why?  Can't I just stream these files directly?

Autobuffer is aware of the bitrate of the video you want to stream and the average throughput of your connection.  Using these two properties, autobuffer builds an appropriately sized buffer such that your video will play without hiccups over your connection. 