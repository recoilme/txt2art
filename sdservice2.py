from diffusers import StableDiffusionXLPipeline, StableDiffusionXLImg2ImgPipeline
from diffusers import EulerAncestralDiscreteScheduler
from torch import float16, cuda

training_refiner_strength = 0.4
base_model_power = 1 - training_refiner_strength
num_inference_steps = 40
stage_1_model_id = '/workspace/ds/models/recoilme-sdxl-v07.safetensors'
torch_device = 'cuda' 

pipe = StableDiffusionXLPipeline.from_single_file(stage_1_model_id, add_watermarker=False, torch_dtype=float16).to(torch_device)
pipe.scheduler = EulerAncestralDiscreteScheduler.from_config(
    pipe.scheduler.config,
)
img2img_pipe = StableDiffusionXLImg2ImgPipeline(
        vae=pipe.vae,
        text_encoder=pipe.text_encoder,
        text_encoder_2=pipe.text_encoder_2,
        tokenizer=pipe.tokenizer,
        tokenizer_2=pipe.tokenizer_2,
        unet=pipe.unet,
        scheduler=pipe.scheduler,

)

prompt = "freckles,long hair,ginger hair,fox ears,cleavage,grey eyes,bondage,upper body,medium breasts,(masterpiece, best quality, very aesthetic, ultra detailed), intricate details, posing, flirty, dynamic"
neg_prompt = "low quality, worst quality, bad hands, crossed eyes, fused fingers, watermark, lowres"
use_zsnr = False

image = pipe(
    prompt=prompt,
    negative_prompt = neg_prompt,
    num_inference_steps=num_inference_steps,
    denoising_end=base_model_power,
    guidance_scale=5.5,
    guidance_rescale=0.7 if use_zsnr else 0.0,
    output_type="latent",
).images
image = img2img_pipe(
    prompt=prompt,
    strength=0.75,
    negative_prompt = neg_prompt,
    num_inference_steps=num_inference_steps,
    denoising_start=base_model_power,
    guidance_scale=1.5,
    guidance_rescale=0.7 if use_zsnr else 0.0,
    image=image,
).images[0]
image.save('demo.png', format="PNG")