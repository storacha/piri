package flatfs

var README_DEF_SHARD = `This is a repository of object data. Each object is in a single file named
<key>.data. Where <key> is a unique identifier for the object. Typically this
repo stores content addressed data, where <key> is a base32 (lowercase, no
padding) encoded multihash.

All the object files are placed in a tree of directories, based on a
function of the key. This function takes the next-to-last two characters of the
key, with underscore padding characters added if the key is too short. This is a
form of sharding similar to the objects directory in git repositories.

For example, an object with a base58 CIDv1 of:

    zb2rhYSxw4ZjuzgCnWSt19Q94ERaeFhu9uSqRgjSdx9bsgM6f

has a base32 (lowercase, no padding) encoded multihash of:

    ciqbvukwqh2atuvj7jcnz6ygxc5sqp5qluuaxju7mik7krx7qwvdeea

and will be placed at:

    ee/ciqbvukwqh2atuvj7jcnz6ygxc5sqp5qluuaxju7mik7krx7qwvdeea.data

with 'ee' being the next-to-last two characters.
`
