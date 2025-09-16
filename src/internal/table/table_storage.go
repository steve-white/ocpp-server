package table

import (
	"context"
	"encoding/json"
	"fmt"

	log "sw/ocpp/csms/internal/logging"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

// Reference: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/data/aztables#section-readme

func GetTableClient(tableName string, accountName string, accountKey string) (*aztables.Client, error) {
	cred, err := aztables.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}
	serviceURL := fmt.Sprintf("https://%s.table.core.windows.net/%s", accountName, tableName)

	client, err := aztables.NewClientWithSharedKey(serviceURL, cred, nil)
	if err != nil {
		panic(err)
	}
	return client, nil
}

// Creates a new table as given in NewClient(). Storage Blob Data Contributor role needed
func CreateTable(client *aztables.Client, tableName string) (*aztables.CreateTableResponse, error) {
	//TODO: Check access policy, Storage Blob Data Contributor role needed
	response, err := client.CreateTable(context.TODO(), &aztables.CreateTableOptions{})
	return &response, err
}

// Adds an entity
func AddEntity[K any](client *aztables.Client, myEntity any) (*aztables.AddEntityResponse, error) {
	marshalled, err := json.Marshal(myEntity)
	if err != nil {
		return nil, err
	}

	log.Logger.Debugf("body: %s", marshalled)
	// TODO: Check access policy, need Storage Table Data Contributor role
	response, err := client.AddEntity(context.TODO(), marshalled, nil)
	return &response, err
}

/*
func listEntities(client *aztables.Client) {
	listPager := client.NewListEntitiesPager(nil)
	pageCount := 0
	for listPager.More() {
		response, err := listPager.NextPage(context.TODO())
		if err != nil {
			panic(err)
		}
		fmt.Printf("There are %d entities in page #%d\n", len(response.Entities), pageCount)
		pageCount += 1
	}
}*/

// Queries an entity for a given partitionKey and rowKey, unmarshalling the JSON in to the given type
// TODO finish - doesnt page properly yet
func QueryEntity[K any](client *aztables.Client, partitionKey string, rowKey string) (map[string]any, error) {
	filter := fmt.Sprintf("PartitionKey eq '%v' and RowKey eq '%v'", partitionKey, rowKey)
	options := &aztables.ListEntitiesOptions{
		Filter: &filter,
		//Select: to.Ptr("RowKey,Price,Inventory,ProductName,OnSale"),
		Top: to.Ptr(int32(15)),
	}

	results := make(map[string]any)
	pager := client.NewListEntitiesPager(options)
	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, entity := range resp.Entities {
			var myEntity K
			err = json.Unmarshal(entity, &myEntity)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Returned [%T]: %s", myEntity, entity)
			key := fmt.Sprintf("%s_%s", partitionKey, rowKey)
			results[key] = myEntity
		}
	}

	return results, nil
}

// Deletes entity with a given partitionKey and rowKey
func DeleteEntity(client *aztables.Client, partitionKey string, rowKey string) (*aztables.DeleteEntityResponse, error) {
	response, err := client.DeleteEntity(context.TODO(), partitionKey, rowKey, nil)
	return &response, err
}

// Deletes the table specified in NewClient()
func DeleteTable(client *aztables.Client) (*aztables.DeleteTableResponse, error) {
	response, err := client.Delete(context.TODO(), nil)
	return &response, err
}
