from http.server import BaseHTTPRequestHandler, HTTPServer
import io
from diffusers import StableDiffusionXLPipeline,StableDiffusionXLImg2ImgPipeline,AutoPipelineForText2Image,AutoPipelineForImage2Image
from sd_embed.embedding_funcs import get_weighted_text_embeddings_sdxl_2p
from sd_embed.embedding_funcs import get_weighted_text_embeddings_sdxl
from diffusers import EulerAncestralDiscreteScheduler
import torch, gc
import base64
import json
import huggingface_hub
import pandas as pd
import numpy as np
import onnxruntime as rt
from PIL import Image
from datetime import datetime
import time

#MODEL_PATH = "/home/recoilme/forge/models/Stable-diffusion/recoilme-sdxl-v09.fp16.safetensors"
MODEL_PATH = "recoilme/recoilme-sdxl-v11"

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

    nsfw = ['explicit', 'sensitive']
    isnsfw = any(key in nsfw for key in rating)

    porn = any(key == 'explicit' for key in rating)
    caption = ", ".join(tags)
    caption = caption.replace("_", " ")
    return isminors, porn, isnsfw, caption
# end wd3

#"/home/recoilme/forge/models/Stable-diffusion/recoilme-sdxl-v09.fp16.safetensors"
#pipe = StableDiffusionXLPipeline.from_pretrained(
pipe = AutoPipelineForText2Image.from_pretrained(
    MODEL_PATH,
    torch_dtype=torch.bfloat16,
    variant="fp16",
    use_safetensors=True
    #enable_pag=True
).to("cuda")
pipe.scheduler = EulerAncestralDiscreteScheduler.from_config(
    pipe.scheduler.config, timestep_spacing="trailing"
)
pipe.enable_vae_slicing()
## Compile the UNet and VAE.
#pipe.unet = torch.compile(pipe.unet, mode="max-autotune", fullgraph=True)
#pipe.vae.decode = torch.compile(pipe.vae.decode, mode="max-autotune", fullgraph=True)

img2img_pipe = AutoPipelineForImage2Image.from_pipe(
    pipe, 
    enable_pag=True
)

img2img_pipe.scheduler = EulerAncestralDiscreteScheduler.from_config(
    img2img_pipe.scheduler.config, timestep_spacing="linspace", use_exponential_sigmas=True
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

def bislerp(samples, width, height):
    def slerp(b1, b2, r):
        '''slerps batches b1, b2 according to ratio r, batches should be flat e.g. NxC'''
        
        c = b1.shape[-1]

        #norms
        b1_norms = torch.norm(b1, dim=-1, keepdim=True)
        b2_norms = torch.norm(b2, dim=-1, keepdim=True)

        #normalize
        b1_normalized = b1 / b1_norms
        b2_normalized = b2 / b2_norms

        #zero when norms are zero
        b1_normalized[b1_norms.expand(-1,c) == 0.0] = 0.0
        b2_normalized[b2_norms.expand(-1,c) == 0.0] = 0.0

        #slerp
        dot = (b1_normalized*b2_normalized).sum(1)
        omega = torch.acos(dot)
        so = torch.sin(omega)

        #technically not mathematically correct, but more pleasing?
        res = (torch.sin((1.0-r.squeeze(1))*omega)/so).unsqueeze(1)*b1_normalized + (torch.sin(r.squeeze(1)*omega)/so).unsqueeze(1) * b2_normalized
        res *= (b1_norms * (1.0-r) + b2_norms * r).expand(-1,c)

        #edge cases for same or polar opposites
        res[dot > 1 - 1e-5] = b1[dot > 1 - 1e-5] 
        res[dot < 1e-5 - 1] = (b1 * (1.0-r) + b2 * r)[dot < 1e-5 - 1]
        return res
    
    def generate_bilinear_data(length_old, length_new, device):
        coords_1 = torch.arange(length_old, dtype=torch.float32, device=device).reshape((1,1,1,-1))
        coords_1 = torch.nn.functional.interpolate(coords_1, size=(1, length_new), mode="bilinear")
        ratios = coords_1 - coords_1.floor()
        coords_1 = coords_1.to(torch.int64)
        
        coords_2 = torch.arange(length_old, dtype=torch.float32, device=device).reshape((1,1,1,-1)) + 1
        coords_2[:,:,:,-1] -= 1
        coords_2 = torch.nn.functional.interpolate(coords_2, size=(1, length_new), mode="bilinear")
        coords_2 = coords_2.to(torch.int64)
        return ratios, coords_1, coords_2

    orig_dtype = samples.dtype
    samples = samples.float()
    n,c,h,w = samples.shape
    h_new, w_new = (height, width)
    
    #linear w
    ratios, coords_1, coords_2 = generate_bilinear_data(w, w_new, samples.device)
    coords_1 = coords_1.expand((n, c, h, -1))
    coords_2 = coords_2.expand((n, c, h, -1))
    ratios = ratios.expand((n, 1, h, -1))

    pass_1 = samples.gather(-1,coords_1).movedim(1, -1).reshape((-1,c))
    pass_2 = samples.gather(-1,coords_2).movedim(1, -1).reshape((-1,c))
    ratios = ratios.movedim(1, -1).reshape((-1,1))

    result = slerp(pass_1, pass_2, ratios)
    result = result.reshape(n, h, w_new, c).movedim(-1, 1)

    #linear h
    ratios, coords_1, coords_2 = generate_bilinear_data(h, h_new, samples.device)
    coords_1 = coords_1.reshape((1,1,-1,1)).expand((n, c, -1, w_new))
    coords_2 = coords_2.reshape((1,1,-1,1)).expand((n, c, -1, w_new))
    ratios = ratios.reshape((1,1,-1,1)).expand((n, 1, -1, w_new))

    pass_1 = result.gather(-2,coords_1).movedim(1, -1).reshape((-1,c))
    pass_2 = result.gather(-2,coords_2).movedim(1, -1).reshape((-1,c))
    ratios = ratios.movedim(1, -1).reshape((-1,1))

    result = slerp(pass_1, pass_2, ratios)
    result = result.reshape(n, h_new, w_new, c).movedim(-1, 1)
    return result.to(orig_dtype)

def txt2img(prompt1,prompt2):
    negative_prompt = "blurry, animation, 3d render, toy, puppet, claymation, low quality, flag, nasa, mission patch, non-paradoxical, loli"
    prompt_embeds, prompt_neg_embeds, pooled_prompt_embeds, negative_pooled_prompt_embeds =  get_weighted_text_embeddings_sdxl(pipe, prompt = prompt1+prompt2, neg_prompt = negative_prompt)
    gc.collect()

    with torch.no_grad():
        images = []
        images.clear()
        generator = torch.Generator()
        generator.manual_seed(int(time.time()))
        
        images = pipe(
            width = 960,#832,1024
            height = 1216,#960,1280
            prompt_embeds=prompt_embeds,
            pooled_prompt_embeds=pooled_prompt_embeds,
            negative_prompt_embeds=prompt_neg_embeds,
            negative_pooled_prompt_embeds=negative_pooled_prompt_embeds,
            num_inference_steps=16,
            guidance_scale=1.5,
            generator=generator,
            num_images_per_prompt=2,
            output_type="latent"
        ).images
        
        has_porn = False    
        if len(images)>0:
            for i in range(1):
                # upscale *1.25
                images = bislerp(images,150,190)

                # restore / add details
                images = img2img_pipe(
                    strength=0.5,#0.12, # strength original image
                    prompt_embeds=prompt_embeds,
                    pooled_prompt_embeds=pooled_prompt_embeds,
                    negative_prompt_embeds=prompt_neg_embeds,
                    negative_pooled_prompt_embeds=negative_pooled_prompt_embeds,
                    num_inference_steps=32,#110,#13 steps, total steps * strength
                    guidance_scale=2.6,
                    pag_scale=1.25,
                    guidance_rescale=0.0,
                    #generator=generator,
                    num_images_per_prompt=len(images),
                    image=images,
                ).images

        has_minors = False
        has_porn = False
        has_nsfw = False
        for i, image in enumerate(images):
            #wd3 
            minors, porn, nsfw, tags = captions(image)
            if minors:
                #print("tags:"+minors+ porn+ nsfw+ tags)
                has_minors = True
            if porn:
                has_porn = True
            if nsfw:
                has_nsfw = True

        if has_minors and has_porn:
            images.clear()
        
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
            prompt2 = ""
            if len(data)>1:
                prompt2 = data['prompt2']
            if len(prompt2)>75:
                prompt1 = ""

            print("propmt:"+datetime.now().strftime("%Y-%m-%d_%H:%M:%S"),"\n", prompt1,"\n", prompt2)  # печатаем строки
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
    text = "a portrait of a pround confident person, an elderly mariner, fully clothed, epic moustache, red rugged sweater, knitted warm cap, with both hands showing a large heavy tropical fish. breathtaking scenery of wildlife lakes. The sun is setting behind him, great light, delicate, shiny, elegant, intricate, rendered in a realistic photo, bold color contrasts, dark background, hyperdetailed, cinematic, dramatic lighting, high resolution, detailed, 4k"
    #text = "Stylized anime art depicting an armored mosquito with metallic plates, cone-shaped stainless steel helm wielding miniature blade against moonlit jungle backdrop."
    #text = "A stunning 4K HDR anime-style illustration of alluring waifu with silvery bob, emerald eyes & porcelain skin in futuristic white dress revealing sculpted shoulders; circuitry patterns hint at her advanced AI nature beneath a human exterior - dreamlike soft focus effect enhances the art's appeal and ambiance ."
    images,pron = txt2img(text,"")
    print("len",len(images))
    if len(images)>1:
        images[0].save(datetime.now().strftime("pron/start_%Y-%m-%d_%H:%M:%S")+'1.jpg')
        images[1].save(datetime.now().strftime("pron/start_%Y-%m-%d_%H:%M:%S")+'2.jpg')
    server_address = ('', port)
    httpd = HTTPServer(server_address, RequestHandler)
    print('Сервер запущен на порту', port)
    httpd.serve_forever()

if __name__ == '__main__':
    run_server(8882)  # замените 8080 на свой порт