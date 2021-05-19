# Payment Channel Diagrams

## How to view
These diagrams were created using the [mermaid diagram live editor](https://mermaid-js.github.io/mermaid-live-editor) which uses this [syntax](https://mermaid-js.github.io/mermaid/#/sequenceDiagram). To view the diagrams below copy the text into the `code` section of the live editor.

### Sequence Diagrams
[setup-update-close.txt](./sequence-diagrams/setup-update-close.txt)
- walks through the steps of the basic case for setting up a channel, updating it, and closing it either cooperatively or non-cooperatively

[withdraw.txt](./sequence-diagrams/withdraw.txt)
- a withdrawal by I after setup and a single update

### State Diagrams
[state-diagrams](./state-diagrams/channel-state.txt)
- state diagram describing the possible channel states and actions that trigger movement to next states
