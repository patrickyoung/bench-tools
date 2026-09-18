#!/usr/bin/env python3
"""Deterministic support-only grayscale conversion of a validated native PNG."""
import struct,zlib
from svg_contract import png_pixels
def grayscale_pixels(source):
    w,h,color,pixels=png_pixels(source)
    c={0:1,2:3,4:2,6:4}[color]; out=bytearray()
    for i in range(0,len(pixels),c):
        r=pixels[i]; g=pixels[i+1] if color in (2,6) else r; b=pixels[i+2] if color in (2,6) else r
        # Integer Rec.709 luma; a review preview, not a color-managed print proof.
        y=(2126*r+7152*g+722*b+5000)//10000
        out.extend((y,y,y,pixels[i+c-1] if color in (4,6) else 255))
    return w,h,6,bytes(out)
def grayscale(source,target):
    w,h,_,pixels=grayscale_pixels(source)
    def chunk(kind,data):
        return struct.pack(">I",len(data))+kind+data+struct.pack(">I",zlib.crc32(kind+data)&0xffffffff)
    raw=b"".join(b"\0"+pixels[j*w*4:(j+1)*w*4] for j in range(h))
    target.write_bytes(b"\x89PNG\r\n\x1a\n"+chunk(b"IHDR",struct.pack(">IIBBBBB",w,h,8,6,0,0,0))
                      +chunk(b"IDAT",zlib.compress(raw,9))+chunk(b"IEND",b""))
