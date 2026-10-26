# Deploy Blister64 ke AWS (EC2 + Cloudflare Tunnel)

Runbook untuk satu server kecil di project AWS (region `ap-southeast-2`, profil CLI `blister64`). Server tidak membuka port masuk sama sekali: pengunjung datang lewat Cloudflare Tunnel, admin masuk lewat Session Manager.

Langkah AWS di bawah belum pernah dijalankan; periksa hasil tiap perintah sebelum lanjut.

## Biaya

Semua ini memotong kredit paket gratis (cek: `aws freetier get-account-plan-state --profile blister64 --region us-east-1`).

- Instance `t4g.small` (2 vCPU ARM, 2 GB RAM) per jam.
- EBS gp3 20 GB per bulan.
- IPv4 publik per jam, dibutuhkan untuk keluar ke internet (Cloudflare, GitHub, Session Manager).
- S3 untuk backup, kecil.

Paket gratis berakhir saat kredit habis atau pada tanggal kedaluwarsanya. Agar server tetap hidup sesudahnya, upgrade ke paid plan di AWS Settings > Billing dan pasang spend limit di sana. Cek angka per bulan di AWS Pricing Calculator untuk Sydney.

## 1. Kode di GitHub

Server mengambil kode dari `github.com/ariefusry/diecast-auction` (private). Beri server akses baca saja lewat deploy key: buat kunci di server (langkah 5), lalu tambahkan public key-nya di GitHub > repo > Settings > Deploy keys, tanpa izin tulis.

## 2. Bucket backup

```bash
P="--profile blister64 --region ap-southeast-2"
ACCOUNT=$(aws sts get-caller-identity $P --query Account --output text)
BUCKET=blister64-backups-$ACCOUNT

aws s3api create-bucket --bucket $BUCKET --create-bucket-configuration LocationConstraint=ap-southeast-2 $P
aws s3api put-public-access-block --bucket $BUCKET $P \
  --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
# Backup lebih tua dari 30 hari dihapus otomatis.
aws s3api put-bucket-lifecycle-configuration --bucket $BUCKET $P --lifecycle-configuration \
  '{"Rules":[{"ID":"expire","Status":"Enabled","Filter":{},"Expiration":{"Days":30}}]}'
```

## 3. Role untuk server

Server memakai role, tidak pernah access key. Role ini hanya boleh dipakai Session Manager dan menulis ke bucket backup.

```bash
aws iam create-role --role-name blister64-server $P --assume-role-policy-document \
  '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"'$ACCOUNT'"}}}]}'
aws iam attach-role-policy --role-name blister64-server $P \
  --policy-arn arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore
aws iam put-role-policy --role-name blister64-server --policy-name backup-write $P --policy-document \
  '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:PutObject","Resource":"arn:aws:s3:::'$BUCKET'/*"}]}'
aws iam create-instance-profile --instance-profile-name blister64-server $P
aws iam add-role-to-instance-profile --instance-profile-name blister64-server --role-name blister64-server $P
```

## 4. Instance

Security group tanpa aturan masuk (tunnel dan Session Manager sama-sama keluar dari server). Amazon Linux 2023 ARM, kredit CPU `standard` (mode default `unlimited` diam-diam menagih CPU surplus), IMDSv2 wajib dengan hop limit 2 agar container tetap bisa memakai role, disk terenkripsi.

```bash
VPC=$(aws ec2 describe-vpcs --filters Name=is-default,Values=true --query 'Vpcs[0].VpcId' --output text $P)
SG=$(aws ec2 create-security-group --group-name blister64-server --description "Blister64: no inbound" \
  --vpc-id $VPC --query GroupId --output text $P)
AMI=$(aws ssm get-parameter --name /aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64 \
  --query Parameter.Value --output text $P)

aws ec2 run-instances $P --image-id $AMI --instance-type t4g.small \
  --security-group-ids $SG --iam-instance-profile Name=blister64-server \
  --associate-public-ip-address \
  --credit-specification CpuCredits=standard \
  --metadata-options HttpTokens=required,HttpPutResponseHopLimit=2,HttpEndpoint=enabled \
  --block-device-mappings '[{"DeviceName":"/dev/xvda","Ebs":{"VolumeSize":20,"VolumeType":"gp3","Encrypted":true}}]' \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=blister64}]' \
  --query 'Instances[0].InstanceId' --output text
```

Masuk ke server (butuh Session Manager plugin untuk AWS CLI di laptop):

```bash
aws ssm start-session --target i-xxxxxxxx $P
```

## 5. Siapkan server

Di dalam sesi Session Manager:

```bash
sudo dnf install -y docker git cronie
sudo systemctl enable --now docker crond
# Compose dan buildx tidak dikemas di AL2023: pasang plugin resmi.
sudo mkdir -p /usr/local/lib/docker/cli-plugins
sudo curl -fsSL -o /usr/local/lib/docker/cli-plugins/docker-compose \
  https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
# buildx bawaan AL2023 (0.12) terlalu lama untuk Compose v5; plugin di /usr/local didahulukan.
V=$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/docker/buildx/releases/latest | sed 's#.*/##')
sudo curl -fsSL -o /usr/local/lib/docker/cli-plugins/docker-buildx \
  https://github.com/docker/buildx/releases/download/$V/buildx-$V.linux-arm64
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-buildx
# Swap 2 GB: build Vite dan Go di RAM 2 GB bisa kehabisan memori.
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile swap swap defaults 0 0' | sudo tee -a /etc/fstab

# Deploy key, lalu tempel public key-nya di GitHub (langkah 1).
ssh-keygen -t ed25519 -N "" -f ~/.ssh/blister64 && cat ~/.ssh/blister64.pub
printf 'Host github.com\n  IdentityFile ~/.ssh/blister64\n' >> ~/.ssh/config
# Tanpa ini clone berhenti di prompt host key. Cocokkan fingerprint dengan docs GitHub (SHA256:+DiY3wvv...).
ssh-keyscan -t ed25519 github.com >> ~/.ssh/known_hosts && ssh-keygen -lf ~/.ssh/known_hosts
sudo mkdir -p /opt/blister64 && sudo chown $USER /opt/blister64
git clone git@github.com:ariefusry/diecast-auction.git /opt/blister64
```

## 6. `.env` produksi

Buat secret baru di server. Jangan salin `.env` dari laptop dan jangan salin volume database dev.

```bash
cd /opt/blister64
cat > .env <<EOF
POSTGRES_USER=diecast
POSTGRES_PASSWORD=$(openssl rand -hex 24)
POSTGRES_DB=diecast
JWT_SECRET=$(openssl rand -hex 32)
DEMO_PASSWORD=$(openssl rand -hex 24)
WEB_PORT=127.0.0.1:8081
PUBLIC_ORIGIN=https://domain-anda
TUNNEL_TOKEN=
GEMINI_API_KEY=
EOF
chmod 600 .env
```

`DEMO_PASSWORD` acak membuat katalog demo tampil, tetapi tidak ada yang bisa masuk sebagai akun demo. `WEB_PORT` di loopback membuat tunnel satu-satunya pintu masuk.

## 7. HTTPS lewat Cloudflare Tunnel

Dengan domain di Cloudflare: Zero Trust > Networks > Tunnels > Create tunnel (Cloudflared), salin token ke `TUNNEL_TOKEN`, lalu tambahkan route Published application `domain-anda` dengan service `http://web:80`. Subdomain dari domain yang sudah dipakai juga bisa (produksi: `blister64.ariefusry.me`, apex tetap di Vercel). Jangan jalankan perintah `docker run ... --token` dari dashboard di laptop: itu menambah konektor kedua, dan sebagian pengunjung jatuh ke laptop. Token yang bocor diganti lewat Rotate token, lalu ambil token baru di Add a replica > Docker.

Tanpa domain, quick tunnel memberi alamat sementara `https://*.trycloudflare.com` yang berganti setiap restart (untuk demo saja). Biarkan `TUNNEL_TOKEN` kosong dan jalankan langkah 8 tanpa `--profile tunnel`, karena service `cloudflared` tanpa token akan restart terus. Setelah stack jalan, nyalakan quick tunnel di latar agar tidak ikut mati saat sesi Session Manager ditutup:

```bash
docker compose --profile tunnel run -d --name blister64-quick cloudflared tunnel --no-autoupdate --url http://web:80
docker logs blister64-quick 2>&1 | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com'
```

Container ini tidak hidup lagi sendiri setelah server reboot. Hapus dengan `docker rm -f blister64-quick`, lalu jalankan ulang (alamatnya akan berganti).

## 8. Jalankan

```bash
cd /opt/blister64
# Variabel buildx: regresi izin baca bake (lihat CLAUDE.md).
BUILDX_BAKE_ENTITLEMENTS_FS=0 docker compose --profile demo --profile tunnel up -d --build
docker compose ps
docker compose logs -f api
```

Dengan quick tunnel, hilangkan `--profile tunnel` dari perintah di atas dan di langkah 10.

Seed berjalan sekali (idempoten) dan mengisi katalog demo.

## 9. Backup harian

```bash
( crontab -l 2>/dev/null; echo "15 3 * * * cd /opt/blister64 && AWS_DEFAULT_REGION=ap-southeast-2 BACKUP_S3_URI=s3://$BUCKET ./scripts/backup.sh >> backups/backup.log 2>&1" ) | crontab -
```

Ganti `$BUCKET` dengan nama bucket dari langkah 2. Uji sekali dengan menjalankan perintah di dalam tanda kutip itu langsung.

Restore ke database yang sedang jalan (menimpa isinya):

```bash
docker compose exec -T db sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner' < backups/db-XXXX.dump
docker compose exec -T api sh -c 'tar -C /data/uploads -xzf -' < backups/uploads-XXXX.tar.gz
```

## 10. Update versi

```bash
cd /opt/blister64 && git pull && BUILDX_BAKE_ENTITLEMENTS_FS=0 docker compose --profile demo --profile tunnel up -d --build
```

Migrasi database berjalan otomatis saat API start.

## 11. Bersihkan

Kalau server tidak dipakai lagi, hapus semuanya agar kredit tidak terpotong. Unduh backup terakhir dulu.

```bash
aws ec2 terminate-instances --instance-ids i-xxxxxxxx $P
aws ec2 delete-security-group --group-id $SG $P            # setelah instance benar-benar terminated
aws iam remove-role-from-instance-profile --instance-profile-name blister64-server --role-name blister64-server $P
aws iam delete-instance-profile --instance-profile-name blister64-server $P
aws iam delete-role-policy --role-name blister64-server --policy-name backup-write $P
aws iam detach-role-policy --role-name blister64-server --policy-arn arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore $P
aws iam delete-role --role-name blister64-server $P
aws s3 rb s3://$BUCKET --force $P                          # menghapus semua backup
```
