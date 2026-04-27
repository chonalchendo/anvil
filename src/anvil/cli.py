from typing import Annotated

import cyclopts

app = cyclopts.App(name="Anvil", help="")
skill_app = cyclopts.App(name="skill", help="")
app.command(skill_app)


app.command
def init(vault: Annotated[str | None, cyclopts.Parameter(name='--vault', help="Set vault location")] = None):
    pass


app.command
def build(issue: Annotated[str, cyclopts.Parameter(name='--issue', help='Select issue to build')]):
    pass


app.command
def compile():
    pass


app.command
def status():
    pass


app.command
def cost():
    pass


app.command
def adopt():
    pass


skill_app.default
def skill():
    pass


skill_app.command
def add():
    pass


skill_app.command
def update():
    pass


skill_app.command
def remove():
    pass


skill_app.command
def list():
    pass

