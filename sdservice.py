from http.server import BaseHTTPRequestHandler, HTTPServer
import io
from diffusers import StableDiffusionXLPipeline
from sd_embed.embedding_funcs import get_weighted_text_embeddings_sdxl_2p
from sd_embed.embedding_funcs import get_weighted_text_embeddings_sdxl
from diffusers import EulerAncestralDiscreteScheduler
import torch, gc
import base64
import json

MODEL_PATH = "/workspace/models/colorfulxl_v74-000012.safetensors"
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

def txt2img(prompt1,prompt2):
    negative_prompt = ""
    prompt_embeds, prompt_neg_embeds, pooled_prompt_embeds, negative_pooled_prompt_embeds =  get_weighted_text_embeddings_sdxl(pipe, prompt = prompt1+prompt2, neg_prompt = negative_prompt)

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


        prompt_embeds, prompt_neg_embeds, pooled_prompt_embeds, negative_pooled_prompt_embeds =  get_weighted_text_embeddings_sdxl_2p(pipe, prompt = prompt1, prompt_2 = prompt1+prompt2, neg_prompt = negative_prompt,neg_prompt_2 = negative_prompt)
        image2 = pipe(
            width = 1216,
            height = 832,
            prompt_embeds=prompt_embeds,
            pooled_prompt_embeds=pooled_prompt_embeds,
            negative_prompt_embeds=prompt_neg_embeds,
            negative_pooled_prompt_embeds=negative_pooled_prompt_embeds,
            num_inference_steps=26,
            guidance_scale=1.5,
            #generator=torch.Generator(device="cuda").seed(),
            num_images_per_prompt=2
        ).images
        images+=image2
        
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
            data = json.loads(body.decode('utf-8'))  # парсим JSON из тела запроса
            prompt1 = data['prompt1']
            prompt2 = data['prompt2']
            print("Получены строки:", prompt1, prompt2)  # печатаем строки
            
    
            images = txt2img(prompt1,prompt2)
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