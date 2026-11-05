"""Builds jigglypuff.ico (PNG-compressed entries) from icon.svg using rsvg-convert."""
import struct, subprocess, sys, tempfile, os

svg, out = sys.argv[1], sys.argv[2]
sizes = [16, 20, 24, 32, 40, 48, 64, 96, 128, 256]
images = []
with tempfile.TemporaryDirectory() as tmp:
    for s in sizes:
        png = os.path.join(tmp, f"{s}.png")
        subprocess.run(["rsvg-convert", "-w", str(s), "-h", str(s), svg, "-o", png], check=True)
        images.append((s, open(png, "rb").read()))
header = struct.pack("<HHH", 0, 1, len(images))
offset = 6 + 16 * len(images)
entries, blobs = b"", b""
for s, data in images:
    dim = 0 if s >= 256 else s
    entries += struct.pack("<BBBBHHII", dim, dim, 0, 0, 1, 32, len(data), offset + len(blobs))
    blobs += data
open(out, "wb").write(header + entries + blobs)
print(f"wrote {out} ({len(images)} sizes)")
