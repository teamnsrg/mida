package resourcetree

import (
	"errors"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/teamnsrg/mida/log"
	t "github.com/teamnsrg/mida/types"
)

// Using information from the Network Domain events, as well as the Debugger Domain
// (for script info), construct a best-effort tree of resources. The idea here is to
// be conservative, so if we are not sure about a resource, favor putting it closer to
// the root of the tree in a more general spot.
func BuildResourceTree(result *t.RawMIDAResult) (*t.ResourceNode, []*t.ResourceNode, error) {

	// First, separate resources into the frames to which they belong
	frameResources := make(map[string]map[string][]network.EventRequestWillBeSent)
	orphans := make(map[string][]network.EventRequestWillBeSent)
	for reqID, reqs := range result.Requests {
		frameID := ""
		for _, req := range reqs {
			if frameID != "" && req.FrameID.String() != frameID {
				log.Log.Warn("Inconsistent frame ID across same request ID")
			}
			frameID = req.FrameID.String()
		}
		if frameID != "" {
			if _, ok := frameResources[frameID]; !ok {
				frameResources[frameID] = make(map[string][]network.EventRequestWillBeSent)
			}

			frameResources[frameID][reqID] = reqs
		} else {
			// For whatever reason, this resource did not have a frame ID, so we just add it as an orphan
			log.Log.Warn("Failed to get frame ID for resource")
			orphans[reqID] = reqs
		}
	}

	var tree []*t.ResourceNode
	if result.FrameTree == nil {
		return nil, tree, errors.New("did not get frame tree from devtools")
	}
	rootNode, tree, err := RecurseFrameResourceTree(result.FrameTree, frameResources, 0)
	if err != nil {
		log.Log.Error(err)
	}

	return rootNode, tree, nil
}

func RecurseFrameResourceTree(frameTree *page.FrameTree, frameResources map[string]map[string][]network.EventRequestWillBeSent, depth int) (*t.ResourceNode, []*t.ResourceNode, error) {
	frameID := frameTree.Frame.ID.String()
	rootNode, orphans, err := GetFrameResourceTree(frameID, frameResources[frameID])
	if err != nil {
		return nil, orphans, err
	}
	if rootNode == nil {
		return nil, orphans, nil
	}

	for _, subTree := range frameTree.ChildFrames {
		_, children, err := RecurseFrameResourceTree(subTree, frameResources, depth+1)
		if err != nil {
			return rootNode, orphans, err
		}
		for _, c := range children {
			rootNode.Children = append(rootNode.Children, c)
		}
	}

	return rootNode, append(orphans, rootNode), nil
}

// Returns a slice of trees from a group of resources within a given frame. Ideally, this will be a small # of
// trees, but if resources cannot be placed, we conservatively return them as their own subtrees.
// NOTE: This function will set the parent nodes for all root nodes it returns (based on the parentNode arg),
// but the caller must add all returned root nodes to the appropriate children array
func GetFrameResourceTree(frameID string, resources map[string][]network.EventRequestWillBeSent) (*t.ResourceNode, []*t.ResourceNode, error) {

	// We generally expect only one document per frame, although some frames have zero documents
	// If there IS a document, it is the root node of our frame
	candidates := make(map[string]bool)
	for reqID, reqs := range resources {
		for _, req := range reqs {
			if req.Type.String() == "Document" {
				candidates[reqID] = true
			}
		}
	}

	// tracks which request IDs have been placed in the tree
	placed := make(map[string]bool)

	// Maps URLs to ResourceNodes for this frame
	// TODO: What happens if a single frame loads the same URL twice?
	urlToNode := make(map[string]*t.ResourceNode)

	// Whether we find a single root node or many candidates, add them to the array of root nodes
	// for the frame that we will return
	var rootNode *t.ResourceNode
	var orphans []*t.ResourceNode
	if len(candidates) == 1 {
		for reqID := range candidates {
			var c []*t.ResourceNode
			newNode := t.ResourceNode{
				RequestID:   reqID,
				FrameID:     frameID,
				IsFrameRoot: true,
				Url:         resources[reqID][len(resources[reqID])-1].Request.URL,
				Children:    c,
			}
			// Mark this particular resource as placed
			rootNode = &newNode
			placed[reqID] = true
			urlToNode[resources[reqID][len(resources[reqID])-1].Request.URL] = rootNode
		}
	} else {
		// There is no root node -- We give up trying to build a tree for this frame
		// and simply return all of the nodes as root nodes
		for reqID := range resources {
			var c []*t.ResourceNode
			newNode := t.ResourceNode{
				RequestID:   reqID,
				FrameID:     frameID,
				IsFrameRoot: false,
				Url:         resources[reqID][len(resources[reqID])-1].Request.URL,
				Children:    c,
			}
			// Don't worry about marking nodes as placed -- we are returning anyway
			// Also don't worry about adding to urlToNode
			orphans = append(orphans, &newNode)
		}
		return nil, orphans, nil
	}

	totalNumPlaced := 0
	for {
		// Count number of nodes placed into the tree in this iteration. If this hits zero, we break
		numPlaced := 0

		for reqID, reqs := range resources {

			// Ignore resources which are already placed
			if _, ok := placed[reqID]; ok {
				continue
			}

			// For now, we look at the final request sent for a particular request ID. There is
			// almost always just a single request per request ID, but there can sometimes be
			// more than one. Seems logical to use the final one sent.
			initiatorUrl := ""
			reqInitiator := reqs[len(reqs)-1].Initiator
			if reqInitiator.Type.String() == "parser" {
				initiatorUrl = reqInitiator.URL
			} else if reqInitiator.Type.String() == "script" {
				// Usually there is not a URL here, but we check just in case
				if reqInitiator.URL != "" {
					initiatorUrl = reqInitiator.URL
				} else {
					// Attempt to extract it from the stack frame. We want the first URL we find
					// as we scan from the top of the stack frame to the bottom.
					for _, callFrame := range reqInitiator.Stack.CallFrames {
						if callFrame.URL != "" {
							initiatorUrl = callFrame.URL
							break
						}
					}
				}
			} else {
				// Some other type of initiator. Just see if there's a URL, and if not, we fail
				if reqInitiator.URL != "" {
					initiatorUrl = reqInitiator.URL
				}
			}

			// If we failed to get an initiator URL, we fail for this node. Add it as a root node and
			// mark it placed.
			if initiatorUrl == "" {
				var c []*t.ResourceNode
				newNode := t.ResourceNode{
					RequestID:   reqID,
					FrameID:     frameID,
					IsFrameRoot: false,
					Url:         "",
					Children:    c,
				}
				orphans = append(orphans, &newNode)
				placed[reqID] = true
				// Don't update urlToNode -- No URL
				continue
			}

			// Determine whether we have a parent node available with the initiatorURL
			if node, ok := urlToNode[initiatorUrl]; ok {
				// If we do, add it as a child to that node and mark it as placed
				var c []*t.ResourceNode
				newNode := t.ResourceNode{
					RequestID:   reqID,
					FrameID:     frameID,
					IsFrameRoot: false,
					Url:         resources[reqID][len(resources[reqID])-1].Request.URL,
					Children:    c,
				}
				node.Children = append(node.Children, &newNode)
				// Mark reqID as placed and add the URL so other nodes can look for it
				placed[reqID] = true
				numPlaced += 1
				urlToNode[resources[reqID][len(resources[reqID])-1].Request.URL] = &newNode
			}
			// If not, we do nothing because we might find it on a subsequent iteration
			// as the tree grows
		}
		// Finish when an iteration over the nodes fails to place any nodes
		if numPlaced == 0 {
			break
		} else {
			totalNumPlaced += numPlaced
		}
	}

	return rootNode, orphans, nil

}
