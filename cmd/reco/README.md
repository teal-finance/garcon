# Change the compression format

The `reco` command is a pure-Go compressor perfect to compress a file from one format to another one.
The name `reco` is from `re-compress` (to compress again).

## Motivation

Linux distributions do not provide tools to manage S2-encoded files.
Thus this tool has been designed to compress a S2-encoded file into another format.

By default, the tool `reco` decompresses the S2-encoded `file.s2` and compresses it to Brotli: `file.br`.

Note: the Brotli, GZip and ZStandard official compressors are faster and perform better compression ratios.

## Supported formats

Format          | Input | Output
--------------- | ------|--------
Brotli          | ✅    | ✅
BZip            | ✅    | ❌
GZip            | ✅    | ✅
S2              | ✅    | ✅
ZStd            | ✅    | ✅
no compression  | ✅    | ✅

## Usage

`go run ./cmd/reco [-h] [-level L] [-loops N] [-v] [INPUT-FILE] [OUTPUT-FILE]`

The compression format is deduced from the file extension (`*.br`, `*.ztd`…).

The `-level L` selects the compression ratio (compression speed).

The `-loops` and `-v` are for bench purpose.

## Bench

The `reco` tool can also [bench](./bench.sh) the compression settings.

Results based on a internal 34 MB file:

| Format | Level | Ratio |   Time    | Ratio/Time |
| :----- | ----: | ----: | :-------: | :--------: |
| S2     |     2 | 60.1% | 00:00.021 |    6.39    |
| S2     |     3 | 63.5% | 00:00.024 |    6.36    |
| ZStd   |     1 | 66.9% | 00:00.095 |    5.79    |
| ZStd   |     2 | 71.3% | 00:00.126 |    5.69    |
| ZStd   |     3 | 74.5% | 00:00.247 |    5.42    |
| Brotli |     5 | 76.5% | 00:01.218 |    4.73    |
| Brotli |     6 | 77.2% | 00:01.619 |    4.61    |
| Brotli |     7 | 77.7% | 00:02.167 |    4.49    |
| Brotli |     8 | 78.0% | 00:03.009 |    4.35    |
| Brotli |     9 | 78.3% | 00:03.714 |    4.26    |
| Brotli |    10 | 80.4% | 00:34.679 |    3.30    |
| Brotli |    11 | 81.4% | 01:28.641 |    2.90    |

The Ratio/Time is `log(ratio/time)`.
