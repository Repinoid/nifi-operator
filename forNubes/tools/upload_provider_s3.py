import configparser
from pathlib import Path

import boto3
from botocore.config import Config

cred_path = Path('/home/naeel/terra/.tmp_aws_credentials')
config = configparser.RawConfigParser()
config.read(cred_path)
key = config.get('registry', 'aws_access_key_id')
secret = config.get('registry', 'aws_secret_access_key')

cfg = Config(signature_version='s3v4', s3={'addressing_style': 'path', 'payload_signing_enabled': False})

s3 = boto3.client(
    's3',
    region_name='us-east-1',
    endpoint_url='https://s3.msk-1.ngcloud.ru',
    aws_access_key_id=key,
    aws_secret_access_key=secret,
    config=cfg,
)

bucket = 'terraform-registry'
prefix = 'terrareg.kube5s.ru/nubes/nubes/2.0.0/'
files = [
    'terraform-provider-nubes_2.0.0_linux_amd64.zip',
    'terraform-provider-nubes_2.0.0_SHA256SUMS',
    'terraform-provider-nubes_2.0.0_SHA256SUMS.sig',
]
base = Path('/home/naeel/terra/nubes_provider_gen')
for name in files:
    path = base / name
    key_name = prefix + name
    with open(path, 'rb') as f:
        s3.put_object(Bucket=bucket, Key=key_name, Body=f)
    print('uploaded', key_name)
