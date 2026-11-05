"""Writes rsrc_windows_amd64.syso (COFF object with a .rsrc section) holding an icon,
a manifest and version info. No dependencies. Usage: mksyso.py <dir> (expects
jigglypuff.ico and jigglypuff.manifest there)."""
import struct, sys

d = sys.argv[1]
LANG = 0x409


def u16(s):
    return s.encode("utf-16-le") + b"\0\0"


def pad4(b):
    return b + b"\0" * (-len(b) % 4)


# ---- icon -> RT_ICON entries + RT_GROUP_ICON
ico = open(f"{d}/jigglypuff.ico", "rb").read()
_, _, count = struct.unpack_from("<HHH", ico, 0)
icons, group = [], struct.pack("<HHH", 0, 1, count)
for i in range(count):
    w, h, cc, r, planes, bpp, size, off = struct.unpack_from("<BBBBHHII", ico, 6 + 16 * i)
    icons.append(ico[off:off + size])
    group += struct.pack("<BBBBHHIH", w, h, cc, r, planes, bpp, size, i + 1)

# ---- version info
def node(key, value=b"", wtype=0, vlen=0, children=()):
    b = pad4(struct.pack("<HHH", 0, vlen, wtype) + u16(key)) + value
    for c in children:
        b = pad4(b) + c
    return struct.pack("<H", len(b)) + b[2:]

strings = {
    "CompanyName": "Kabir", "FileDescription": "Jigglypuff - PC cleaner", "FileVersion": "1.0.0",
    "InternalName": "Jigglypuff", "LegalCopyright": "(c) 2026 Kabir", "OriginalFilename": "Jigglypuff.exe",
    "ProductName": "Jigglypuff", "ProductVersion": "1.0.0",
}
table = node("040904B0", wtype=1, children=[
    node(k, u16(v), 1, len(v) + 1) for k, v in strings.items()])
fixed = struct.pack("<13I", 0xFEEF04BD, 0x00010000, 0x00010000, 0, 0x00010000, 0,
                    0x3F, 0, 0x40004, 1, 0, 0, 0)
version = node("VS_VERSION_INFO", fixed, 0, len(fixed), [
    node("StringFileInfo", wtype=1, children=[table]),
    node("VarFileInfo", wtype=1, children=[node("Translation", struct.pack("<HH", LANG, 1200), 0, 4)]),
])

manifest = open(f"{d}/jigglypuff.manifest", "rb").read()

# type -> [(id, data)], sorted by type id
res = {3: [(i + 1, b) for i, b in enumerate(icons)], 14: [(1, group)], 16: [(1, version)], 24: [(1, manifest)]}

# ---- lay out the resource tree: dirs, then data entries, then data
def dirsize(n):
    return 16 + 8 * n

types = sorted(res)
off = dirsize(len(types))
name_dir_off = {}
for t in types:
    name_dir_off[t] = off
    off += dirsize(len(res[t]))
lang_dir_off = {}
for t in types:
    for rid, _ in res[t]:
        lang_dir_off[(t, rid)] = off
        off += dirsize(1)
entry_off = {}
for t in types:
    for rid, _ in res[t]:
        entry_off[(t, rid)] = off
        off += 16
data_off = {}
for t in types:
    for rid, blob in res[t]:
        off += -off % 8
        data_off[(t, rid)] = off
        off += len(blob)
size = off + (-off % 8)

buf = bytearray(size)
def dirhdr(at, n):
    struct.pack_into("<IIHHHH", buf, at, 0, 0, 0, 0, 0, n)

dirhdr(0, len(types))
for i, t in enumerate(types):
    struct.pack_into("<II", buf, 16 + 8 * i, t, 0x80000000 | name_dir_off[t])
    dirhdr(name_dir_off[t], len(res[t]))
    for j, (rid, blob) in enumerate(res[t]):
        struct.pack_into("<II", buf, name_dir_off[t] + 16 + 8 * j, rid, 0x80000000 | lang_dir_off[(t, rid)])
        dirhdr(lang_dir_off[(t, rid)], 1)
        struct.pack_into("<II", buf, lang_dir_off[(t, rid)] + 16, LANG, entry_off[(t, rid)])
        struct.pack_into("<IIII", buf, entry_off[(t, rid)], data_off[(t, rid)], len(blob), 0, 0)
        buf[data_off[(t, rid)]:data_off[(t, rid)] + len(blob)] = blob

# ---- COFF object: header, one section, raw data, relocations, symbol table
relocs = sorted(entry_off.values())
raw_ptr = 20 + 40
rel_ptr = raw_ptr + size
sym_ptr = rel_ptr + 10 * len(relocs)
out = struct.pack("<HHIIIHH", 0x8664, 1, 0, sym_ptr, 1, 0, 0)
out += struct.pack("<8sIIIIIIHHI", b".rsrc", 0, 0, size, raw_ptr, rel_ptr, 0, len(relocs), 0, 0x40000040)
out += bytes(buf)
for r in relocs:  # IMAGE_REL_AMD64_ADDR32NB against symbol 0
    out += struct.pack("<IIH", r, 0, 3)
out += struct.pack("<8sIhHBB", b".rsrc", 0, 1, 0, 3, 0)  # static section symbol
out += struct.pack("<I", 4)  # empty string table
open(f"{d}/rsrc_windows_amd64.syso", "wb").write(out)
print(f"wrote syso: {len(icons)} icons, {size} bytes of resources")
