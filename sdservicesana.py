from http.server import BaseHTTPRequestHandler, HTTPServer
import io
import torch, gc
import base64
import json
import numpy as np
from PIL import Image
from datetime import datetime
import time

import torch
from app.sana_pipeline import SanaPipeline

device = torch.device("cuda:0" if torch.cuda.is_available() else "cpu")

sana = SanaPipeline("configs/sana_config/1024ms/Sana_1600M_img1024_AdamW.yaml")
sana.from_pretrained("/home/recoilme/models/sana-1.6-v10.pth")

    
def encode_images_to_base64(images):
    encoded_images = []
    for i, image in enumerate(images):
        with io.BytesIO() as buffer:
            image.save(buffer, format='JPEG', quality=97)
            encoded_image = base64.b64encode(buffer.getvalue()).decode('utf-8')
            encoded_images.append(encoded_image)
        del buffer  # удалить буфер
        del image  
    gc.collect() 
    del images  # удалить images
    return json.dumps(encoded_images)
    
def norm_ip(img, low, high):
    img.clamp_(min=low, max=high)
    img.sub_(low).div_(max(high - low, 1e-5))
    return img

@torch.inference_mode()
def txt2img(prompt1,prompt2):
    negative_prompt = "bad anatomy, extra limbs, low quality"

    with torch.no_grad():
        images = []
        images.clear()
        device = torch.device("cuda:0" if torch.cuda.is_available() else "cpu")
        generator = torch.Generator(device=device).manual_seed(int(time.time()))

        images_np = sana(
            prompt=prompt1+prompt2,
            negative_prompt = negative_prompt,
            height=1280,
            width=1024,
            guidance_scale=4.5,
            #pag_guidance_scale=1.5,
            num_inference_steps=24,
            generator=generator,
        )
        images = [
            Image.fromarray(
                norm_ip(img, -1, 1)
                .mul(255)
                .add_(0.5)
                .clamp_(0, 255)
                .permute(1, 2, 0)
                .to("cpu", torch.uint8)
                .numpy()
                .astype(np.uint8)
            )
            for img in images_np
        ]
        #torch.cuda.empty_cache()
        gc.collect()
        has_porn = False
        return images,has_porn

class RequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        try:
            content_length = int(self.headers['Content-Length'])
            body = self.rfile.read(content_length)
            data = json.loads(body.decode('utf-8'))  # парсим JSON из тела запроса
            prompt1 = data['prompt1']
            prompt2 = ""
            if len(data)>1:
                prompt2 = data['prompt2']
            if len(prompt2)>75:
                prompt1 = ""
            images,has_porn = txt2img(prompt1,prompt2)
            if len(images)>0:
                if has_porn:
                    self.send_response(210)
                else:
                    self.send_response(200)
            else:
                self.send_response(204)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            if len(images)>0:
                result = encode_images_to_base64(images)
                self.wfile.write(result.encode('utf-8'))
            del body
            del prompt1
            del prompt2
        except Exception as e:
            print(f"Ошибка: {e}")
            self.send_response(500)
            self.send_header('Content-type', 'text/plain')
            self.end_headers()
            self.wfile.write(b"")

def run_server(port):
    #warmup
    text = "Stylized anime art depicting an armored mosquito with metallic plates, cone-shaped stainless steel helm wielding miniature blade against moonlit jungle backdrop."
    images,pron = txt2img(text,"")
    print("len",len(images))
    if len(images)>0:
        images[0].save(datetime.now().strftime("start_%Y-%m-%d_%H:%M:%S")+'1.jpg')
    print("starting")
    server_address = ('', port)
    httpd = HTTPServer(server_address, RequestHandler)
    print('Started', port)
    httpd.serve_forever()

if __name__ == '__main__':
    run_server(8887)  # замените 8080 на свой порт