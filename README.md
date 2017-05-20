This program helps me deal with duplicate files.

I am trying to organize my collection of images. It is disorganized and
contains many duplicates. I want to delete duplicates so I can have less
to organize.


# What this program does
This program takes a directory and calculates checksums for each file under
it. It then checks whether any two files have the same checksum, and if so
it reports the files as duplicates.

You can provide rules to define which file to keep in order to delete one
of them. This only runs in live mode. By default it will only report the
duplicates.


# Defining rules
You define rules by writing a JSON file. Each rule specifies which file to
remove by identifying which directory holds the file to keep and which
holds the one to delete. This allows fine grained control.

An example rule file with one rule looks like this:

```
{
  "rules": [
    {
      "keep":   "/directory1",
      "remove": "/directory2"
  ]
}
```

In this case, if we detect duplicate files `/directory1/example.png` and
`/directory2/example-test.png`, the program deletes
`/directory2/example-test.png` and keeps the other.


# Behaviour in more detail
  - Recursively find all files.
  - Calculate the checksum of each file.
  - Check whether any two files have the same checksum.
  - If they do, check whether the two files are really identical.
  - If they are, take action. This may be to just report (in non-live mode) or
    to remove one of them (in live mode).
  - Report any two files with identical checksums.
  - Report any two files with identical names.
