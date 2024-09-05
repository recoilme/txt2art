from http.server import BaseHTTPRequestHandler, HTTPServer
import io
from diffusers import StableDiffusionXLPipeline
from sd_embed.embedding_funcs import get_weighted_text_embeddings_sdxl
from diffusers import EulerAncestralDiscreteScheduler
import torch, gc
import base64
import json

MODEL_PATH = "/workspace/ds/models/colorfulxl_v74-000016.safetensors"
pipe = StableDiffusionXLPipeline.from_single_file(
    MODEL_PATH,
    torch_dtype=torch.bfloat16,
    variant="fp16",
    use_safetensors=True
).to('cuda')

pipe.scheduler = EulerAncestralDiscreteScheduler.from_config(
    pipe.scheduler.config,
)
pipe.enable_vae_slicing()
## Compile the UNet and VAE.
#pipe.unet = torch.compile(pipe.unet, mode="max-autotune", fullgraph=True)
#pipe.vae.decode = torch.compile(pipe.vae.decode, mode="max-autotune", fullgraph=True)
    
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

def txt2img(prompt):
    negative_prompt = ""
    prompt_embeds, prompt_neg_embeds, pooled_prompt_embeds, negative_pooled_prompt_embeds =  get_weighted_text_embeddings_sdxl(pipe, prompt = prompt, neg_prompt = negative_prompt)
    with torch.no_grad():
        images = pipe(
            width = 832,
            height = 1216,
            prompt_embeds=prompt_embeds,
            pooled_prompt_embeds=pooled_prompt_embeds,
            negative_prompt_embeds=prompt_neg_embeds,
            negative_pooled_prompt_embeds=negative_pooled_prompt_embeds,
            num_inference_steps=26,
            guidance_scale=1.5,
            #generator=torch.Generator(device="cuda").seed(),
            num_images_per_prompt=2
        ).images
        image = pipe(
            width = 1280,
            height = 768,
            prompt_embeds=prompt_embeds,
            pooled_prompt_embeds=pooled_prompt_embeds,
            negative_prompt_embeds=prompt_neg_embeds,
            negative_pooled_prompt_embeds=negative_pooled_prompt_embeds,
            num_inference_steps=26,
            guidance_scale=1.5,
            #generator=torch.Generator(device="cuda").seed(),
            num_images_per_prompt=1
        ).images
        images+=image
        del prompt_embeds, prompt_neg_embeds, pooled_prompt_embeds, negative_pooled_prompt_embeds
        gc.collect()
        return images

class RequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        try:
            content_length = int(self.headers['Content-Length'])
            body = self.rfile.read(content_length)
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            prompt = body.decode('utf-8')
            print("Получена строка:", prompt)  # печатаем строку
    
            images = txt2img(prompt)
            json = encode_images_to_base64(images)
            self.wfile.write(json.encode('utf-8'))
            del body
            del prompt
        except Exception as e:
            print(f"Ошибка: {e}")
            self.send_response(500)
            self.send_header('Content-type', 'text/plain')
            self.end_headers()
            self.wfile.write(b"")

def run_server(port):
    prompt = 'test'
    print("test строка:", prompt)  # печатаем строку

    #warmup
    #images = txt2img("prompt")
    
    server_address = ('', port)
    httpd = HTTPServer(server_address, RequestHandler)
    print('Сервер запущен на порту', port)
    httpd.serve_forever()

if __name__ == '__main__':
    run_server(8882)  # замените 8080 на свой порт