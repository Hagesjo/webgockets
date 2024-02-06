# Webgockets

A websocket client implementation in go.

## Example

A simple echo example

```
c, err := NewClient("ws://example.com")
if err != nil {
    // Handle error.
}

if err := c.Connect(); err != nil {
    // Handle error.
}

for {
    var closeErr *webgockets.ErrClose
    bs, err := c.Read()
    if errors.As(err, &closeErr) { // Connection closed.
        break
    } else if err != nil {
        // Handle error.
    }

    if _, err := c.Write(bs); err != nil && errors.Is(err, io.EOF) { // Connection closed.
        break
    } else if err != nil {
        // Handle error.
    }
}
```

## Autobahn Testsuite

This implementation passes all basic cases in the test suite (except for UTF-8 checks, I deemed that unnecessary strict).
The report is [here](https://hagesjo.github.io/webgockets/)
