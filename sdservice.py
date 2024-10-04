from http.server import BaseHTTPRequestHandler, HTTPServer
import io
from diffusers import StableDiffusionXLPipeline,StableDiffusionXLImg2ImgPipeline
from sd_embed.embedding_funcs import get_weighted_text_embeddings_sdxl_2p
from sd_embed.embedding_funcs import get_weighted_text_embeddings_sdxl
from diffusers import EulerAncestralDiscreteScheduler
import torch, gc
import base64
import json
from RealESRGAN import RealESRGAN
import huggingface_hub
import pandas as pd
import numpy as np
import onnxruntime as rt
from PIL import Image
from datetime import datetime

#https://github.com/ai-forever/Real-ESRGAN?tab=readme-ov-file
modelr = RealESRGAN("cuda", scale=2)
modelr.load_weights('weights/RealESRGAN_x2.pth', download=True)
#wd3 tagger
# Specific model repository from SmilingWolf's collection / Repository Default vit tagger v3
VIT_MODEL_DSV3_REPO = "SmilingWolf/wd-vit-tagger-v3"
MODEL_FILENAME = "model.onnx"
LABEL_FILENAME = "selected_tags.csv"

class LabelData:
    def __init__(self, names, rating, general, character):
        self.names = names
        self.rating = rating
        self.general = general
        self.character = character
        
# Download the model and labels
def download_model(model_repo):
    csv_path = huggingface_hub.hf_hub_download(model_repo, LABEL_FILENAME)
    model_path = huggingface_hub.hf_hub_download(model_repo, MODEL_FILENAME)
    return csv_path, model_path

def load_model_and_tags(model_repo):
    csv_path, model_path = download_model(model_repo)
    df = pd.read_csv(csv_path)
    tag_data = LabelData(
        names=df["name"].tolist(),
        rating=list(np.where(df["category"] == 9)[0]),
        general=list(np.where(df["category"] == 0)[0]),
        character=list(np.where(df["category"] == 4)[0]),
    )
    model = rt.InferenceSession(model_path)
    target_size = model.get_inputs()[0].shape[2]
    
    return model, tag_data, target_size

model, tag_data, target_size = load_model_and_tags(VIT_MODEL_DSV3_REPO)

# Image preprocessing function / Memproses gambar
def prepare_image(image, target_size):
    canvas = Image.new("RGBA", image.size, (255, 255, 255))
    canvas.paste(image, mask=image.split()[3] if image.mode == 'RGBA' else None)
    image = canvas.convert("RGB")

    # Pad image to a square
    max_dim = max(image.size)
    pad_left = (max_dim - image.size[0]) // 2
    pad_top = (max_dim - image.size[1]) // 2
    padded_image = Image.new("RGB", (max_dim, max_dim), (255, 255, 255))
    padded_image.paste(image, (pad_left, pad_top))

    # Resize
    padded_image = padded_image.resize((target_size, target_size), Image.BICUBIC)

    # Convert to numpy array
    image_array = np.asarray(padded_image, dtype=np.float32)[..., [2, 1, 0]]
    
    return np.expand_dims(image_array, axis=0) # Add batch dimension

# Function to tag all images in a directory and save the captions / Fitur untuk tagging gambar dalam folder dan menyimpan caption dengan file .txt
def process_predictions_with_thresholds(preds, tag_data, character_thresh, general_thresh, rating_thresh):
    # Extract prediction scores
    scores = preds.flatten()
    
    # Filter and sort character and general tags based on thresholds / Filter dan pengurutan tag berdasarkan ambang batas
    character_tags = [tag_data.names[i] for i in tag_data.character if scores[i] >= character_thresh]
    general_tags = [(tag_data.names[i], scores[i]) for i in tag_data.general if scores[i] >= general_thresh]
    general_tags = sorted(general_tags, key=lambda x: x[1], reverse=True)

    rating_tags = [(tag_data.names[i], scores[i]) for i in tag_data.rating if scores[i] >= rating_thresh]
    rating_tags = sorted(rating_tags, key=lambda x: x[1], reverse=True)
    rating_tags = [key for key, value in rating_tags]
    #next(iter(sorted(rating_tags, key=lambda x: x[1], reverse=True)), '')
    #rating = ""
    #if len(rating_first)>0:
    #    rating = rating_first[0]
        
    # Sort tags based on user preference / Mengurutkan tags berdasarkan keinginan pengguna
    final_tags = []
    final_tags = [key for key, value in general_tags]
    final_tags.extend([key for key, value in character_tags])
    return rating_tags, final_tags

def captions(image):
    character_thresh=0.85
    general_thresh=0.35
    rating_thresh=0.5
    processed_image = prepare_image(image, target_size)
    preds = model.run(None, {model.get_inputs()[0].name: processed_image})[0]
    rating, tags = process_predictions_with_thresholds(preds, tag_data, character_thresh, general_thresh, rating_thresh)
    minors = ['loli', 'child','small_breasts','flatchested'] 
    isminors = any(key in minors for key in tags)

    nsfw = ['explicit', 'sensitive','questionable']
    isnsfw = any(key in nsfw for key in rating)

    porn = any(key == 'explicit' for key in rating)
    caption = ", ".join(tags)
    caption = caption.replace("_", " ")
    return isminors, porn, isnsfw, caption
# end wd3

MODEL_PATH = "/home/recoilme/forge/models/Stable-diffusion/recoilme-sdxl-v09.fp16.safetensors"
#MODEL_PATH = "/workspace/recoilme-sdxl-v09.fp16.safetensors"#"/home/recoilme/forge/models/Stable-diffusion/recoilme-sdxl-v09.fp16.safetensors"
#pipe = StableDiffusionXLPipeline.from_pretrained(
pipe = StableDiffusionXLPipeline.from_single_file(
    MODEL_PATH,
    torch_dtype=torch.bfloat16,
    variant="bf16",
    use_safetensors=True
).to("cuda")
pipe.scheduler = EulerAncestralDiscreteScheduler.from_config(
    pipe.scheduler.config,
)
pipe.enable_vae_slicing()
## Compile the UNet and VAE.
#pipe.unet = torch.compile(pipe.unet, mode="max-autotune", fullgraph=True)
#pipe.vae.decode = torch.compile(pipe.vae.decode, mode="max-autotune", fullgraph=True)

img2img_pipe = StableDiffusionXLImg2ImgPipeline.from_pipe(
    pipe
)

img2img_pipe.enable_model_cpu_offload()
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
    negative_prompt = "worst quality, low quality, text, censored, deformed, bad hand, blurry, watermark, multiple phones, weights, bunny ears, extra hands, extra fingers, deformed fingers"
    prompt_embeds, prompt_neg_embeds, pooled_prompt_embeds, negative_pooled_prompt_embeds =  get_weighted_text_embeddings_sdxl(pipe, prompt = prompt1+prompt2, neg_prompt = negative_prompt)

    gc.collect()
    #torch.cuda.empty_cache()

    with torch.no_grad():
        images = pipe(
            width = 832,
            height = 960,
            prompt_embeds=prompt_embeds,
            pooled_prompt_embeds=pooled_prompt_embeds,
            negative_prompt_embeds=prompt_neg_embeds,
            negative_pooled_prompt_embeds=negative_pooled_prompt_embeds,
            num_inference_steps=20,
            guidance_scale=5,
            #generator=torch.Generator(device="cuda").seed(),
            num_images_per_prompt=2
        ).images
        #images[0].save('0.png')

        has_minors = False
        has_porn = False
        has_nsfw = False
        for i, image in enumerate(images):
            #wd3 
            minors, porn, nsfw, tags = captions(image)
            #print(tags)
            if minors:
                has_minors = True
            if porn:
                has_porn = True
            if nsfw:
                has_nsfw = True
            
            predicted_image = modelr.predict(images[i])
            images[i] = predicted_image.resize((int(predicted_image.width * 0.75), int(predicted_image.height * 0.75)))#0.625

        if has_minors and (has_nsfw or has_porn):
            images[0].save(datetime.now().strftime("pron/%Y-%m-%d_%H:%M:%S")+'.jpg')
            images = []

        if len(images)>0:
            images = img2img_pipe(
                strength=0.7,
                steps_offset = 500,
                prompt_embeds=prompt_embeds,
                pooled_prompt_embeds=pooled_prompt_embeds,
                negative_prompt_embeds=prompt_neg_embeds,
                negative_pooled_prompt_embeds=negative_pooled_prompt_embeds,
                num_inference_steps=50,
                guidance_scale=5,
                guidance_rescale=0.0,
                num_images_per_prompt=2,
                image=images,
            ).images
        
        del prompt_embeds, prompt_neg_embeds, pooled_prompt_embeds, negative_pooled_prompt_embeds
        gc.collect()
        return images,has_porn

class RequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        try:
            content_length = int(self.headers['Content-Length'])
            body = self.rfile.read(content_length)
            data = json.loads(body.decode('utf-8'))  # парсим JSON из тела запроса
            prompt1 = data['prompt1']
            prompt2 = data['prompt2']
            print("propmt:", prompt1, prompt2)  # печатаем строки
            images,has_porn = txt2img(prompt1,prompt2)
            self.send_header('Content-type', 'application/json')
            if len(images)>0:
                result = encode_images_to_base64(images)
                self.wfile.write(result.encode('utf-8'))
                if has_porn:
                    self.send_response(210)
                else:
                    self.send_response(200)
            else:
                self.send_response(204)
            self.end_headers()
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
    images,pron = txt2img("1girl,oral,sex","")
    print("len",len(images))
    if len(images)>0:
        images[0].save(datetime.now().strftime("pron/start_%Y-%m-%d_%H:%M:%S")+'.jpg')
    server_address = ('', port)
    httpd = HTTPServer(server_address, RequestHandler)
    print('Сервер запущен на порту', port)
    httpd.serve_forever()

if __name__ == '__main__':
    run_server(8882)  # замените 8080 на свой порт