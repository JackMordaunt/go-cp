# `cp`

Package `cp` is a tiny utility. It exposes a type that can easily copy one file directory into another, somewhat analogous to the `cp` unix command.

In natural Go fashion the files are copied concurrently to maximise throughput. 

The `Copier` can be used as-is.

You can plugin any file system you want using the `github.com/spf13/afero.Fs` interface. The OS filesystem object is the default.

## Usage

```go
func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		oops("not enough arguments\n")
	}
	from, to := args[0], args[1]
	copier := cp.Copier{
		Clobber: true,
	}
	if err := copier.Copy(from, to); err != nil {
		fatal("copying files: %v\n", err)
	}
}
```