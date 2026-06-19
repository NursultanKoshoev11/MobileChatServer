# Media moderation support

Public requests now accept up to 3 photos and 1 video in the structured request body.

The server validates media size, builds a moderation summary, and sends media posts through the moderation pipeline so they are reviewed instead of bypassing filters.
