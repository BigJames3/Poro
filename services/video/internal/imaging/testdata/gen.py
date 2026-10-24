"""Regenerates the photo fixtures (Pillow): python3 gen.py

Each oriented image stores, before orientation, a 64x32 blue picture with a
red 16x16 block in its top-left corner, an EXIF orientation and GPS
coordinates (Abidjan), as a phone would write them.
"""
from PIL import Image

RED, BLUE = (220, 20, 20), (20, 40, 220)


def picture(w=64, h=32, mode="RGB"):
    img = Image.new(mode, (w, h), BLUE + ((255,) if mode == "RGBA" else ()))
    for x in range(16):
        for y in range(16):
            img.putpixel((x, y), RED + ((255,) if mode == "RGBA" else ()))
    return img


def exif(orientation):
    e = Image.Exif()
    e[0x0112] = orientation
    gps = e.get_ifd(0x8825)
    gps[1], gps[2] = "N", (5.0, 21.0, 36.0)
    gps[3], gps[4] = "W", (4.0, 0.0, 29.0)
    return e


for o in range(1, 9):
    picture().save(f"jpeg_o{o}.jpg", "JPEG", quality=95, exif=exif(o))
picture().save("png_o6.png", "PNG", exif=exif(6))
picture().save("webp_o6.webp", "WEBP", lossless=True, exif=exif(6))

alpha = picture(mode="RGBA")
for x in range(32, 64):
    for y in range(32):
        alpha.putpixel((x, y), (0, 0, 0, 0))  # right half transparent
alpha.save("png_alpha.png", "PNG")
alpha.save("webp_alpha.webp", "WEBP", lossless=True)

Image.new("RGB", (3000, 1000), BLUE).save("big.jpg", "JPEG", quality=80)
Image.new("RGB", (800, 600), BLUE).save("small.jpg", "JPEG", quality=80)
