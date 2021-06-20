DRIVE_NAME=$1

sudo parted $DRIVE_NAME mklabel gpt
sudo parted $DRIVE_NAME mkpart primary fat32 0% 1GB
sudo parted $DRIVE_NAME mkpart primary ext4 1GB 101GB
sudo parted $DRIVE_NAME mkpart primary 101GB 100%
sudo mkfs.vfat -n system-boot -F 32 "${DRIVE_NAME}1"
sudo mkfs.ext4 -L writable "${DRIVE_NAME}2"

sudo mkdir /mnt/{boot-img,rootfs-img,boot-drive,rootfs-drive}
LOOP_DEVICE=$(sudo losetup -fP --show ubuntu-20.10-server-arm64-raspi.img)
sudo mount -o noatime "${LOOP_DEVICE}p1" /mnt/boot-img
sudo mount -o noatime "${LOOP_DEVICE}p2" /mnt/rootfs-img
sudo mount -o noatime "${DRIVE_NAME}1" /mnt/boot-drive
sudo mount -o noatime "${DRIVE_NAME}2" /mnt/rootfs-drive
sudo rsync -axv /mnt/boot-img/ /mnt/boot-drive
sudo rsync -axv /mnt/rootfs-img/ /mnt/rootfs-drive
sudo touch /mnt/boot-drive/ssh
sudo cp -f user-data-rpi112 /mnt/boot-drive/user-data
sudo umount /mnt/boot-img
sudo umount /mnt/rootfs-img
sudo umount /mnt/boot-drive
sudo umount /mnt/rootfs-drive