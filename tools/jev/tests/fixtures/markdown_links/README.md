# Markdown link syntax

[Repository](../../../../../README.md#readme)
[Root page](/README.md)
[Documentation directory](../../../../../docs/)
[Query and anchor](../../../../../README.md?view=1#readme)
[Angle destination](<../../../../../README.md> "Repository page")
[Full reference][root]
[root][]
[root]
[![Linked image](../../../../../README.md)](../../../../../README.md)
[Local anchor](#local-anchor)

[root]: ../../../../../README.md
[//]: # (A bare anchor comment is valid.)

```markdown
[Code example](this-example-is-not-a-link.md)
~~~
```

~~~markdown
![Code example](this-is-not-an-image.png)
~~~

[Canonical repository](https://github.com/Sidiora-Labs/paxeer-monorepo)
![Canonical badge](https://img.shields.io/github/license/Sidiora-Labs/paxeer-monorepo)

<a href="/README.md?view=1&amp;plain=1#readme">HTML link</a>
<img
  src="/tools/jev/tests/fixtures/markdown_links/Link%20target.svg" alt="Percent-encoded image">
![Space in destination](</tools/jev/tests/fixtures/markdown_links/Link target.svg>)

`[Inline code](this-is-not-a-link.md)`
<!-- [Comment](this-is-not-a-link-either.md) -->

## Local anchor

The examples above supply Markdown syntax to the link parser, including literal
anchor tokens and destinations interpreted relative to the repository root.
