from PIL import Image
import os

src = "/workspace/site/static/logo.png"
out_dir = "/workspace/site/static"

with Image.open(src) as img:
    base = img.convert("RGBA")

    sizes_png = {
        "favicon-16x16.png": (16, 16),
        "favicon-32x32.png": (32, 32),
        "favicon-96x96.png": (96, 96),
        "apple-touch-icon.png": (180, 180),
        "android-chrome-192x192.png": (192, 192),
        "android-chrome-512x512.png": (512, 512),
    }
    for name, (w, h) in sizes_png.items():
        out = base.resize((w, h), Image.LANCZOS)
        out.save(os.path.join(out_dir, name), "PNG")
        print(f"Created {name}")

    icon_sizes = [(16, 16), (32, 32), (48, 48), (64, 64)]
    imgs = [base.resize(s, Image.LANCZOS) for s in icon_sizes]
    imgs[0].save(
        os.path.join(out_dir, "favicon.ico"),
        format="ICO",
        sizes=icon_sizes,
        append_images=imgs[1:],
    )
    print("Created favicon.ico (multi-size)")

print("Done")
