# go-torrent

Bencode and torrent metainfo parsing for Go. No dependencies outside the
standard library.

Written as a replacement for `github.com/anacrolix/torrent/metainfo` and its
`bencode` package in [autobrr](https://github.com/autobrr/autobrr), where
pulling in a full torrent client module was too heavy and its strict parsing
rejected torrents that trackers actually serve. The packages keep the same
names and layout, so migrating is usually just replacing the module prefix
`github.com/anacrolix/torrent` with `github.com/autobrr/go-torrent`.

The scope is read-only on purpose: parse torrents, get the infohash, sizes,
files and trackers. There is no torrent creation, no piece hashing and no
magnet link support. BitTorrent v2 and hybrid torrents are supported for
parsing, including correct sizes and piece-aligned file offsets from the v2
file tree.

## Install

```
go get github.com/autobrr/go-torrent
```

## Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/autobrr/go-torrent/metainfo"
)

func main() {
	mi, err := metainfo.LoadFromFile("debian-12.5.0-amd64-netinst.iso.torrent")
	if err != nil {
		log.Fatal(err)
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(info.BestName())
	fmt.Println(mi.HashInfoBytes()) // v1 infohash as hex
	fmt.Println(info.TotalLength()) // size in bytes

	for _, tier := range mi.UpvertedAnnounceList() {
		for _, tracker := range tier {
			fmt.Println(tracker)
		}
	}

	for _, f := range info.UpvertedFiles() {
		fmt.Println(f.DisplayPath(&info), f.Length)
	}
}
```

Plain bencode lives in `github.com/autobrr/go-torrent/bencode` with the usual
`Marshal`, `Unmarshal`, `Decoder` and `Encoder` API and `bencode:"..."`
struct tags, including `omitempty` and `ignore_unmarshal_type_error`:

```go
var v struct {
	Name  string `bencode:"name"`
	Total int64  `bencode:"total,omitempty"`
}
err := bencode.Unmarshal(data, &v)
```

## Differences from anacrolix/torrent

The parser is lenient about the malformed optional fields trackers produce
in the wild. Only the info dict is treated as canonical.

- `announce-list` may be a bencode string, a flat list of strings, or
  contain junk entries; all of it decodes without error. The motivating case
  was a private tracker encoding an empty announce-list as the bencode
  string `0:`, which fails the entire parse in anacrolix.
- Junk in optional metadata such as `creation date`, `comment` or
  `created by` never fails a parse, whatever type or value it holds.
- Trailing whitespace after the torrent payload is accepted. Some tracker
  webservers append a newline.
- Unsorted and duplicate dict keys are accepted (the last value wins), as
  are leading zeros in integer literals and string length prefixes.

It is stricter where safety matters, since torrent files are usually
untrusted input:

- nesting is capped at 512 levels
- integers must fit the target type; there is no big.Int fallback
- a lying length prefix cannot force allocation beyond the actual input
- the v2 file tree decodes in a single pass with memory linear in input
  size

Error types keep the anacrolix names and shapes (`bencode.SyntaxError`,
`bencode.UnmarshalTypeError`, `bencode.ErrUnusedTrailingBytes`, ...), so
existing `errors.As` code ports unchanged.

## Performance

Measured with benchstat (n=6, go 1.26, Ryzen 3700X) against
`github.com/anacrolix/torrent` v1.61.0, decoding real torrent files.
"Pipeline" is the full consumer path: load, unmarshal the info dict,
compute the infohash, sum the file lengths.

| Benchmark                     | anacrolix | this package | delta |
| ----------------------------- | --------- | ------------ | ----- |
| Load, 287 B torrent           | 8.3 us    | 2.5 us       | -70%  |
| Load, 2.8 KB                  | 26.3 us   | 6.9 us       | -74%  |
| Load, 31 KB                   | 28.0 us   | 21.3 us      | -24%  |
| Load, 123 KB                  | 104.9 us  | 91.7 us      | -13%  |
| UnmarshalInfo, v2 hybrid      | 118.3 us  | 36.8 us      | -69%  |
| Pipeline, 123 KB single file  | 212.1 us  | 196.9 us     | -7%   |
| Pipeline, v2 hybrid           | 240.8 us  | 153.4 us     | -36%  |
| Decode into `any`, 335 KB     | 227.5 us  | 78.0 us      | -66%  |

Geomean across the suite: 44% less time, 47% fewer allocations, 28% fewer
bytes allocated. `Unmarshal` decodes directly from the input slice, so the
info dict is captured as a subslice instead of being copied through an
intermediate buffer.

One caveat: loading v2 hybrid torrents allocates more bytes here (+68%)
than anacrolix. That is the cost of the single-pass file tree decoding,
which trades a bounded constant factor for the unbounded amplification the
recursive approach allows. It is still faster in wall time.

## Testing

The test suite asserts byte-level parity with anacrolix on a corpus of real
torrents: identical infohashes, names, sizes, file lists, offsets and
byte-identical re-encoded info dicts. Behavior was additionally checked
with a differential fuzzer against anacrolix and with adversarial tests for
hostile input (deep nesting, lying length prefixes, memory amplification).
The bencode fuzzer also verifies that the slice fast path and the streaming
decoder always agree.

## License

MIT
